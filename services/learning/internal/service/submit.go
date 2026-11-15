package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"ego/platform/httpx"
	"ego/platform/valuex"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"

	"gorm.io/datatypes"
)

// SubmitAttempt là API nộp tay. Lỗi chấm điểm do đề thiếu dữ liệu được trả 400 để client
// biết bài nộp không hợp lệ, thay vì 500.
func (s *service) SubmitAttempt(ctx context.Context, userID string, attemptID uint, req dto.SubmitAttemptRequest) (*dto.AttemptResponse, *httpx.HTTPError) {
	response, err := s.submitAttempt(ctx, userID, attemptID, &req)
	if err != nil {
		if errors.Is(err, dto.ErrScoreQuestionNotFound) || errors.Is(err, dto.ErrScoreQuestionSectionEmpty) {
			return nil, httpx.NewError(http.StatusBadRequest, err.Error(), err)
		}
		return nil, httpx.FromError(err, "[ERROR] Failed to submit attempt")
	}
	s.attachClassroom(ctx, response)
	return response, nil
}

// submitAttempt chấm điểm và chốt một lượt làm bài, dùng cho mọi đường nộp bài bình thường:
//   - Nộp tay (submitted khác nil): đáp án trong body thay toàn bộ đáp án đã lưu, vì đó là
//     trạng thái cuối cùng học viên thấy trên màn hình.
//   - Server tự nộp khi hết giờ, kể cả nộp "lười" khi người dùng quay lại sau giờ (nil):
//     chấm lại các đáp án đã lưu dần qua WebSocket trong lúc làm.
//
// Gọi lại trên bài đã nộp trả lại kết quả cũ thay vì lỗi, để nộp trùng (bấm hai lần, worker
// và người dùng cùng nộp) không làm hỏng gì. Logic nằm ở finishAttempt.
func (s *service) submitAttempt(ctx context.Context, userID string, attemptID uint, submitted *dto.SubmitAttemptRequest) (*dto.AttemptResponse, error) {
	return s.finishAttempt(ctx, userID, attemptID, submitted, nil, "")
}

// Lý do server tự nộp bài, gửi kèm ATTEMPT_SUBMITTED để client báo cho học sinh trước
// khi chuyển sang trang kết quả.
const (
	SubmitReasonExamUpdated      = "EXAM_UPDATED"
	SubmitReasonAssignmentPaused = "ASSIGNMENT_PAUSED"
	SubmitReasonRemovedFromClass = "REMOVED_FROM_CLASS"
)

// submitPayload dựng payload của ATTEMPT_SUBMITTED; reason chỉ có khi server tự nộp, để client
// báo cho học viên lý do bài bị nộp.
func submitPayload(attemptID uint, reason string) map[string]any {
	payload := map[string]any{"attemptId": attemptID}
	if reason != "" {
		payload["reason"] = reason
	}
	return payload
}

// submitAttemptWithReason là submitAttempt bằng đáp án đã lưu, kèm lý do cho sự kiện socket.
func (s *service) submitAttemptWithReason(ctx context.Context, userID string, attemptID uint, reason string) (*dto.AttemptResponse, error) {
	return s.finishAttempt(ctx, userID, attemptID, nil, nil, reason)
}

// attemptTermination là thông tin đình chỉ khi finishAttempt chốt bài vì vi phạm: bài vẫn
// được chấm (RawScore, để còn khôi phục được) nhưng Score = 0. actor là "system" hoặc ID
// giáo viên.
type attemptTermination struct {
	reason model.IntegrityTerminationReason
	actor  string
}

// finishAttempt là phần chung của nộp bài và đình chỉ: chấm điểm, chốt trạng thái, lưu bản
// chụp lịch sử và cập nhật bài nộp của lớp trong một transaction, rồi báo realtime sau commit.
// termination khác nil là đình chỉ; đình chỉ chỉ áp được cho bài còn đang làm, bài đã nộp
// hoặc đã hủy trả ErrAttemptAlreadySubmitted / ErrAttemptCancelled. reason là lý do server
// tự nộp, gửi kèm sự kiện cho client.
func (s *service) finishAttempt(ctx context.Context, userID string, attemptID uint, submitted *dto.SubmitAttemptRequest, termination *attemptTermination, reason string) (*dto.AttemptResponse, error) {
	var response *dto.AttemptResponse
	var gradebook *gradebookEvent
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		attempt, err := repo.GetAttemptForLifecycle(ctx, attemptID, userID)
		if err != nil {
			return err
		}

		// Bài đã nộp: nộp lại thì trả kết quả cũ (idempotent), nhưng đình chỉ thì báo lỗi vì
		// không được đổi kết quả của bài đã chốt bằng đường này. Bài đã hủy không nộp được.
		if termination != nil && attempt.Status == model.AttemptStatusSubmitted {
			return ErrAttemptAlreadySubmitted
		}
		switch attempt.Status {
		case model.AttemptStatusSubmitted:
			if attempt.History == nil {
				return ErrAttemptHistoryNotFound
			}
			response, err = dto.UnmarshalAttemptHistory(attempt.History.Data)
			return err
		case model.AttemptStatusCancelled:
			return ErrAttemptCancelled
		case model.AttemptStatusInProgress:
		default:
			return ErrAttemptNotActive
		}
		// Nộp tay chỉ hợp lệ trước khi hết giờ, để không ai sửa đáp án sau giờ bằng cách gửi
		// body; hết giờ thì phải để server nộp bằng đáp án đã lưu.
		now := s.clock()
		var submittedAnswers *[]dto.SubmitAttemptAnswer
		if submitted != nil {
			if attempt.ExpiresAt != nil && !now.Before(*attempt.ExpiresAt) {
				return ErrAttemptExpired
			}
			// Bài có giám sát: chỉ phiên đang giữ (token của lần claim gần nhất) được nộp
			// kèm đáp án, như requireSessionOwner ở đường WS (kiểm sau hết giờ như ở đó).
			if attempt.Proctored && attempt.SessionTokenHash != nil && !sessionTokenMatches(*attempt.SessionTokenHash, submitted.SessionToken) {
				return ErrAttemptSessionInUse
			}
			submittedAnswers = &submitted.Answers
		}
		// TOEIC_STRICT: cập nhật vị trí trang nghe tới hiện tại rồi lọc body, để không sửa được
		// đáp án của trang nghe đã qua (đang khóa) bằng cách gửi kèm khi nộp.
		if submittedAnswers != nil && isStrict(attempt) {
			advanceListening(attempt, now, s.listeningGrace())
			filtered := strictSubmittedAnswers(attempt, *submittedAnswers)
			submittedAnswers = &filtered
		}

		// Chấm lại từng đáp án theo đáp án đúng hiện tại của đề, không tin IsCorrect đã lưu.
		// Chỉ các câu trong phạm vi của lượt làm bài (section/part đã chốt lúc tạo) mới được
		// chấm, nên tra câu hỏi qua map này thay vì cả đề.
		questions := attemptQuestions(attempt)

		answers := make([]model.AttemptAnswer, 0)
		if submittedAnswers != nil {
			// Nộp tay: body là nguồn sự thật, dựng lại toàn bộ đáp án từ body.
			answers = make([]model.AttemptAnswer, 0, len(*submittedAnswers))
			for _, item := range *submittedAnswers {
				// Câu không thuộc phạm vi bài là client gửi sai hoặc cố gửi câu ngoài phạm vi:
				// từ chối cả lần nộp để client biết mà sửa, không âm thầm bỏ qua.
				question := questions[item.QuestionID]
				if question == nil {
					return ErrQuestionNotBelongToAttempt
				}
				// Câu gửi lên nhưng chưa chọn đáp án coi như bỏ trống: không lưu dòng nào, câu
				// đó tính sai khi chấm.
				if item.SelectedOptionID == nil {
					continue
				}

				// Đáp án phải là option của chính câu đó, tránh gửi ID option đúng của câu khác.
				selectedOption := findOption(question, *item.SelectedOptionID)
				if selectedOption == nil {
					return ErrOptionNotBelongToQuestion
				}

				// Đúng/sai lấy từ đề tại thời điểm nộp, không nhận từ client. Body không mang
				// thời điểm chọn đáp án nên ghi bằng lúc nộp.
				isCorrect := selectedOption.IsCorrect
				answeredAt := now
				answers = append(answers, model.AttemptAnswer{
					AttemptID:        attempt.ID,
					QuestionID:       question.ID,
					SelectedOptionID: item.SelectedOptionID,
					IsCorrect:        &isCorrect,
					AnsweredAt:       &answeredAt,
				})
			}
		} else {
			// Server tự nộp: chấm lại các đáp án đã lưu dần qua WebSocket. Không có người dùng
			// để báo lỗi, nên dòng nào không còn hợp lệ thì bỏ qua (câu đó tính sai) thay vì
			// làm hỏng cả lần nộp và để bài kẹt ở trạng thái đang làm.
			answers = make([]model.AttemptAnswer, 0, len(attempt.Answers))
			for _, savedAnswer := range attempt.Answers {
				// Dòng đã lưu nhưng học viên bỏ chọn sau đó.
				if savedAnswer.SelectedOptionID == nil {
					continue
				}
				// Câu không còn trong phạm vi bài (đề bị sửa giữa chừng).
				question := questions[savedAnswer.QuestionID]
				if question == nil {
					continue
				}

				// Option không còn thuộc câu (đề bị sửa giữa chừng).
				selectedOption := findOption(question, *savedAnswer.SelectedOptionID)
				if selectedOption == nil {
					continue
				}

				// Chấm lại theo đáp án đúng hiện tại, để đính chính đề trước lúc nộp được áp
				// dụng. Giữ thời điểm chọn đáp án thật; dữ liệu cũ thiếu thì dùng lúc nộp.
				isCorrect := selectedOption.IsCorrect
				answeredAt := savedAnswer.AnsweredAt
				if answeredAt == nil {
					value := now
					answeredAt = &value
				}
				answers = append(answers, model.AttemptAnswer{
					AttemptID:        attempt.ID,
					QuestionID:       question.ID,
					SelectedOptionID: savedAnswer.SelectedOptionID,
					IsCorrect:        &isCorrect,
					AnsweredAt:       answeredAt,
				})
			}
		}

		// Xóa hết đáp án cũ rồi ghi bộ vừa chấm, để bảng đáp án đúng bằng tập đã dùng tính
		// điểm: không còn dòng bỏ chọn, dòng câu ngoài phạm vi hay IsCorrect cũ sai lệch.
		if err := repo.ReplaceAttemptAnswers(ctx, attempt.ID, answers); err != nil {
			return err
		}

		// Đếm câu đúng và tính điểm; câu bị loại khỏi điểm (đính chính đề) không được tính.
		correctAnswers := 0
		for _, answer := range answers {
			question := questions[answer.QuestionID]
			if question != nil && !question.ExcludedFromScore && answer.IsCorrect != nil && *answer.IsCorrect {
				correctAnswers++
			}
		}

		score, err := dto.CalculateAttemptScore(attempt.Exam.Type, answers, questions, attempt.TotalQuestions)
		if err != nil {
			return err
		}
		score = valuex.RoundFloat(score, 2)

		// Đình chỉ ghi lại lúc, lý do và người đình chỉ; applyScore ép Score = 0 nhưng vẫn giữ
		// RawScore. finalizeAttempt chỉ chốt được nếu bài còn đang làm, để hai lần nộp cùng lúc
		// chỉ một lần thắng.
		attempt.SubmittedAt = &now
		if termination != nil {
			reason := termination.reason
			attempt.IntegrityOutcome = model.IntegrityOutcomeTerminated
			attempt.TerminatedAt = &now
			attempt.TerminationReason = &reason
			attempt.TerminatedBy = &termination.actor
		}
		applyScore(attempt, correctAnswers, score)
		attempt.Answers = answers
		if err := finalizeAttempt(ctx, repo, attempt); err != nil {
			return err
		}

		// Lưu bản chụp để xem lại bài không phụ thuộc vào việc đề bị sửa về sau.
		data, err := dto.MarshalAttemptHistory(attempt)
		if err != nil {
			return err
		}
		history := &model.History{
			Data:      datatypes.JSON(data),
			AttemptID: attempt.ID,
			ExamTitle: attempt.Exam.Title,
			ExamType:  attempt.Exam.Type,
			ExamYear:  attempt.Exam.Year,
		}
		if err := repo.UpsertHistory(ctx, history); err != nil {
			return err
		}
		attempt.History = history
		if attempt.ContextType == model.AttemptContextTypeClassroomAssignment && attempt.ContextID != nil && attempt.Score != nil {
			// Bài tập lớp: ghi điểm vào bài nộp cùng transaction để gradebook không bao giờ lệch
			// với kết quả bài làm, và xếp thông báo vào outbox để gửi sau.
			gradebook, err = updateAssignmentSubmissionResult(ctx, repo, attempt)
			if err != nil {
				return err
			}
			title, body := "Assignment graded", "Your assignment was submitted and graded"
			if termination != nil {
				title, body = "Assignment suspended", "Your assignment was suspended for violating the exam rules"
			}
			if err := createNotificationOutbox(ctx, repo, model.NotificationSourceAttempt, fmt.Sprintf("attempt:%d:submitted", attempt.ID), []string{userID},
				title, body, "ATTEMPT", attempt.ID, 0, time.Time{}); err != nil {
				return err
			}
		}
		response = dto.ToAttemptHistoryResponse(attempt)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Bài đã chốt thì không còn phiên nào giữ quyền làm bài nữa.
	s.clearAttemptOwner(attemptID)
	if gradebook != nil {
		s.publishGradebookEvents([]*gradebookEvent{gradebook})
	}
	// Báo gradebook và ATTEMPT_SUBMITTED chỉ sau khi commit, để không đẩy sự kiện của một
	// transaction có thể bị rollback. Client khóa bài và chuyển sang kết quả khi nhận sự kiện.
	if s.attemptPublisher != nil && response != nil {
		payload := submitPayload(attemptID, reason)
		payload["score"] = response.Score
		s.attemptPublisher(userID, map[string]any{"type": "ATTEMPT_SUBMITTED", "payload": payload})
	}
	return response, nil
}

// strictSubmittedAnswers chặn lách khóa qua body submit ở flow strict: câu đang khóa giữ
// đáp án đã lưu (bỏ qua body), câu đang mở lấy theo body; câu mở mà body không nhắc tới
// giữ đáp án đã lưu (client chỉ biết đáp án của các trang đã tải). Câu chưa tới trang
// không có đáp án đã lưu nên bị bỏ qua và tính sai. Câu không thuộc attempt giữ lại để
// kiểm tra phía sau trả ErrQuestionNotBelongToAttempt như cũ.
func strictSubmittedAnswers(attempt *model.Attempt, submitted []dto.SubmitAttemptAnswer) []dto.SubmitAttemptAnswer {
	result := make([]dto.SubmitAttemptAnswer, 0, len(submitted)+len(attempt.Answers))
	inBody := make(map[uint]bool, len(submitted))
	for _, item := range submitted {
		if pageOfQuestion(attempt, item.QuestionID) < 0 || questionAccess(attempt, item.QuestionID) {
			result = append(result, item)
			inBody[item.QuestionID] = true
		}
	}
	for _, saved := range attempt.Answers {
		if saved.SelectedOptionID == nil || pageOfQuestion(attempt, saved.QuestionID) < 0 || inBody[saved.QuestionID] {
			continue
		}
		result = append(result, dto.SubmitAttemptAnswer{QuestionID: saved.QuestionID, SelectedOptionID: saved.SelectedOptionID})
	}
	return result
}

// eachScopedQuestion duyệt các câu của exam nằm trong phạm vi của lượt làm bài và gọi fn cho
// từng câu. fn trả false để dừng sớm.
// Phạm vi gồm hai điều kiện, câu phải thỏa cả hai:
//   - sectionCode khác nil: chỉ lấy câu của section có mã đó (vd chỉ LISTENING).
//   - partCodes không rỗng: chỉ lấy câu có part nằm trong danh sách (vd part 5, 6).
//     Câu không có part thì bị loại.
//
// Bỏ trống cả hai là lấy cả đề.
// Đây là nơi duy nhất giữ quy tắc phạm vi, để đếm số câu, chấm điểm, chia trang và xem lại
// luôn dựa trên cùng một tập câu.
func eachScopedQuestion(exam *model.Exam, sectionCode *model.SectionCode, partCodes []model.PartCode, fn func(*model.Question) bool) {
	inScope := func(question *model.Question) bool {
		return len(partCodes) == 0 || (question.Part != nil && partInSlice(*question.Part, partCodes))
	}
	// Mỗi section duyệt câu đứng riêng trước, rồi tới câu trong các group (đoạn nghe/đọc dùng chung).
	for sectionIdx := range exam.Sections {
		section := &exam.Sections[sectionIdx]
		if sectionCode != nil && section.Code != *sectionCode {
			continue
		}
		for questionIdx := range section.Questions {
			if question := &section.Questions[questionIdx]; inScope(question) && !fn(question) {
				return
			}
		}
		for groupIdx := range section.Groups {
			group := &section.Groups[groupIdx]
			for questionIdx := range group.Questions {
				if question := &group.Questions[questionIdx]; inScope(question) && !fn(question) {
					return
				}
			}
		}
	}
}

// attemptQuestions là các câu thuộc attempt, theo ID.
func attemptQuestions(attempt *model.Attempt) map[uint]*model.Question {
	questions := make(map[uint]*model.Question, attempt.TotalQuestions)
	eachScopedQuestion(&attempt.Exam, attempt.SectionCode, attempt.PartCodes, func(question *model.Question) bool {
		questions[question.ID] = question
		return true
	})
	return questions
}

// findAttemptQuestion tìm một câu thuộc attempt, dừng ở câu khớp (đường WS gửi đáp án).
func findAttemptQuestion(attempt *model.Attempt, questionID uint) *model.Question {
	var found *model.Question
	eachScopedQuestion(&attempt.Exam, attempt.SectionCode, attempt.PartCodes, func(question *model.Question) bool {
		if question.ID == questionID {
			found = question
			return false
		}
		return true
	})
	return found
}

// findOption tìm option theo ID trong chính câu hỏi; nil nghĩa là option không thuộc câu đó.
func findOption(question *model.Question, optionID uint) *model.Option {
	for i := range question.Options {
		if question.Options[i].ID == optionID {
			return &question.Options[i]
		}
	}
	return nil
}
