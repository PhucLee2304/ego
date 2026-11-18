package service

import (
	"context"
	"errors"
	"time"

	"ego/platform/valuex"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/socket"

	"gorm.io/gorm"
)

// Nối attempt ↔ assignment submission, thay 3 RPC exams→classrooms
// (ValidateAssignmentAttempt, CreateAssignmentSubmission, UpdateAssignmentSubmissionResult).
// Submission được ghi trong tx của attempt nên hai bên commit/rollback cùng nhau;
// GRADEBOOK_UPDATED do caller gửi sau khi commit.

// validateAssignmentAttempt khóa assignment (FOR SHARE, gọi trong tx tạo attempt) và kiểm
// tra học sinh được làm assignment này với đề examID tại thời điểm now. Attempt được dựng
// từ chính bản đã khóa này (thời lượng, hạn nộp, cấu hình giám sát), nên giáo viên sửa
// assignment đồng thời thì hoặc chờ tx này xong, hoặc tx này thấy bản đã sửa.
func validateAssignmentAttempt(ctx context.Context, repo repository.Repository, userID string, assignmentID, examID uint, now time.Time) (*model.Assignment, error) {
	assignment, err := repo.GetAssignmentForAttemptValidationForShare(ctx, assignmentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAssignmentNotFound
		}
		return nil, err
	}

	if assignment.ExamID != examID {
		return nil, ErrAssignmentExamMismatch
	}
	if !assignment.Classroom.Active {
		return nil, ErrClassroomNotActive
	}
	if !assignment.Active {
		return nil, ErrAssignmentNotActive
	}

	isApproved, err := repo.IsStudentApprovedInClassroom(ctx, assignment.ClassroomID, userID)
	if err != nil {
		return nil, err
	}
	if !isApproved {
		return nil, ErrStudentNotApproved
	}

	if now.Before(assignment.OpensAt) {
		return nil, ErrAssignmentNotOpen
	}
	if now.After(assignment.DueAt) {
		return nil, ErrAssignmentClosed
	}
	return assignment, nil
}

// createAssignmentSubmission ghi bài nộp của học sinh cho bài tập, trỏ tới lượt làm bài vừa
// tạo, ngay khi học sinh bắt đầu làm. Nhờ đó bảng điểm của giáo viên thấy học sinh đang làm
// (chưa có điểm) chứ không phải chưa bắt đầu.
// Gọi trong cùng transaction với việc tạo attempt, để không bao giờ có lượt làm bài tập mà
// thiếu bài nộp hay ngược lại. assignment là bản validateAssignmentAttempt đã khóa trong tx đó.
// Trả sự kiện GRADEBOOK_UPDATED để caller gửi cho giáo viên sau khi commit.
func createAssignmentSubmission(ctx context.Context, repo repository.Repository, assignment *model.Assignment, studentID string, attemptID uint, now time.Time) (*gradebookEvent, error) {
	// Upsert theo (assignment, học sinh): mỗi học sinh chỉ có một bài nộp cho mỗi bài tập. Đã có
	// dòng thì chỉ trỏ lại attempt_id sang lượt mới, không tạo thêm dòng.
	submission := &model.AssignmentSubmission{AssignmentID: assignment.ID, StudentID: studentID, AttemptID: attemptID}
	if err := repo.UpsertAssignmentSubmission(ctx, submission); err != nil {
		return nil, err
	}
	// Trạng thái gửi cho giáo viên tính từ bài nộp và hạn nộp: vừa bắt đầu (chưa điểm) là đang làm.
	return gradebookUpdatedEvent(assignment, submission, model.Status(submission, now, assignment.DueAt)), nil
}

// updateAssignmentSubmissionResult ghi điểm (0–10, làm tròn 2 chữ số) của attempt
// đã nộp vào submission. Không kiểm tra lớp/assignment còn hoạt động: bài nộp lúc
// tắt assignment hoặc xóa học sinh vẫn phải có điểm.
func updateAssignmentSubmissionResult(ctx context.Context, repo repository.Repository, attempt *model.Attempt) (*gradebookEvent, error) {
	assignment, err := repo.GetAssignmentForAttemptValidation(ctx, *attempt.ContextID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAssignmentNotFound
		}
		return nil, err
	}
	return writeAssignmentSubmissionResult(ctx, repo, assignment, attempt)
}

// writeAssignmentSubmissionResult như updateAssignmentSubmissionResult nhưng dùng
// assignment caller đã có (kèm Classroom), để reconcile không đọc lại cho từng bài.
func writeAssignmentSubmissionResult(ctx context.Context, repo repository.Repository, assignment *model.Assignment, attempt *model.Attempt) (*gradebookEvent, error) {
	if *attempt.Score < 0 || *attempt.Score > 10 {
		return nil, ErrInvalidSubmissionScore
	}
	score := valuex.RoundFloat(*attempt.Score, 2)
	submission := &model.AssignmentSubmission{
		AssignmentID: assignment.ID,
		StudentID:    attempt.UserID,
		AttemptID:    attempt.ID,
		Score:        &score,
	}
	if err := repo.UpsertAssignmentSubmissionResult(ctx, submission); err != nil {
		return nil, err
	}
	return gradebookUpdatedEvent(assignment, submission, model.SubmissionStatusSubmitted), nil
}

// gradebookEvent là một sự kiện GRADEBOOK_UPDATED đang chờ gửi, kèm người nhận. Được dựng bên
// trong transaction (lúc tạo hoặc chấm bài nộp) nhưng chỉ gửi bằng publishGradebookEvents sau
// khi commit, để giáo viên không thấy một thay đổi rồi bị rollback.
//   - teacherID: giáo viên của lớp, người duy nhất nhận sự kiện (học sinh không xem bảng điểm).
//   - event: nội dung gửi qua WebSocket.
type gradebookEvent struct {
	teacherID string
	event     socket.GradebookUpdatedEvent
}

// gradebookUpdatedEvent dựng sự kiện GRADEBOOK_UPDATED cho một dòng của bảng điểm, để màn bảng
// điểm giáo viên đang mở cập nhật đúng ô của học sinh đó ngay khi học sinh bắt đầu làm, nộp bài,
// bị đình chỉ hay được chấm lại, không phải tải lại cả bảng.
// assignment phải có sẵn Classroom (để lấy giáo viên nhận sự kiện). status do caller tính, vì nó
// phụ thuộc thời điểm và hạn nộp. Payload mang đủ khóa (lớp, bài tập, học sinh) để client tìm
// đúng ô, cùng attempt, trạng thái và điểm mới.
func gradebookUpdatedEvent(assignment *model.Assignment, submission *model.AssignmentSubmission, status model.SubmissionStatus) *gradebookEvent {
	return &gradebookEvent{teacherID: assignment.Classroom.TeacherID, event: socket.GradebookUpdatedEvent{
		Type: socket.EventGradebookUpdated,
		Payload: socket.GradebookUpdatedPayload{
			ClassroomID:  assignment.ClassroomID,
			AssignmentID: assignment.ID,
			StudentID:    submission.StudentID,
			AttemptID:    submission.AttemptID,
			Status:       string(status),
			Score:        submission.Score,
			UpdatedAt:    time.Now(),
		},
	}}
}

// publishGradebookEvents gửi các sự kiện GRADEBOOK_UPDATED đã gom tới giáo viên tương ứng qua
// WebSocket, để bảng điểm đang mở cập nhật ngay. Chỉ gọi sau khi transaction ghi bài nộp đã
// commit: gửi trước commit thì giáo viên có thể thấy một thay đổi rồi bị rollback.
// Gửi kiểu fire-and-forget: không trả lỗi, vì bảng điểm vẫn đúng khi giáo viên tải lại.
func (s *service) publishGradebookEvents(events []*gradebookEvent) {
	// Gửi qua hub /ws/classrooms (giáo viên đang mở màn lớp học). Không có hub (test, hoặc chưa
	// cấu hình) thì bỏ qua.
	if s.roomPublisher == nil {
		return
	}
	for _, event := range events {
		s.roomPublisher(event.teacherID, event.event)
	}
}

// assignmentAttemptExpiresAt tính hạn kết thúc của lượt làm bài tập: bắt đầu + thời lượng,
// nhưng không quá hạn nộp của assignment, để bắt đầu sát hạn không được làm quá hạn.
func assignmentAttemptExpiresAt(startedAt time.Time, durationMinutes int, dueAt time.Time) time.Time {
	expiresAt := startedAt.Add(time.Duration(durationMinutes) * time.Minute)
	if dueAt.Before(expiresAt) {
		return dueAt
	}
	return expiresAt
}
