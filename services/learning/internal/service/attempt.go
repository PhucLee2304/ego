package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/platform/valuex"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CreateAttempt tạo lượt làm bài mới cho userID trên đề examID. Mỗi người chỉ có một
// lượt đang làm tại một thời điểm, và mỗi bài tập lớp chỉ được làm một lần. Mọi thứ
// thuộc về lượt làm bài (thời gian, phạm vi câu hỏi, luồng làm bài, cấu hình giám sát,
// bài nộp của lớp) được chốt ngay lúc tạo, trong một transaction, để các thay đổi về sau
// của đề hay bài tập không làm lệch bài đang làm. Luồng TOEIC_STRICT chỉ trả thông tin
// lượt làm bài (client lấy từng trang sau); các luồng khác trả kèm toàn bộ câu hỏi.
func (s *service) CreateAttempt(ctx context.Context, userID string, examID uint, req dto.CreateExamAttemptRequest) (*dto.AttemptResponse, *httpx.HTTPError) {
	activeAttempt, activeErr := s.repo.GetActiveAttemptByUserID(ctx, userID)
	// Lượt đang làm đã hết giờ nhưng worker chưa kịp nộp thì nộp luôn ở đây, để người
	// dùng không bị chặn tạo lượt mới vì một bài thực chất đã kết thúc. Còn giờ thì bỏ
	// qua, bước kiểm tra trong transaction bên dưới sẽ trả lỗi.
	// Việc nộp cố ý chạy trong transaction riêng và commit trước: nếu gộp vào transaction
	// tạo lượt mới thì khi bước tạo lỗi, bài hết giờ cũng bị rollback về chưa nộp.
	if activeErr == nil && activeAttempt.ExpiresAt != nil && !s.clock().Before(*activeAttempt.ExpiresAt) {
		if _, err := s.submitAttempt(ctx, userID, activeAttempt.ID, nil); err != nil &&
			!errors.Is(err, ErrAttemptAlreadySubmitted) {
			return nil, httpx.FromError(err, "[ERROR] Failed to create attempt")
		}
	} else if activeErr != nil && !errors.Is(activeErr, gorm.ErrRecordNotFound) {
		return nil, httpx.FromError(activeErr, "[ERROR] Failed to create attempt")
	}

	var attempt *model.Attempt
	var gradebook *gradebookEvent
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		// Đọc lại lượt đang làm (không dùng kết quả ở trên, vì bài hết giờ có thể vừa được
		// nộp): còn lượt chưa hết giờ thì không cho tạo thêm. Hai request tạo cùng lúc vẫn
		// có thể lọt qua bước này; unique index lượt đang làm chặn ở bước insert.
		if _, err := repo.GetActiveAttemptByUserID(ctx, userID); err == nil {
			return ErrActiveAttemptExists
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		now := s.clock()
		// Bài tập lớp: khóa assignment trong tx để thời lượng, hạn nộp và cấu hình giám sát
		// chụp vào lượt làm bài đúng với bản hiện hành, không bị giáo viên sửa chen giữa.
		// Bài có giám sát thì học viên phải đồng ý đúng phiên bản điều khoản mới được làm.
		var assignment *model.Assignment
		if req.ContextType == model.AttemptContextTypeClassroomAssignment {
			locked, err := validateAssignmentAttempt(ctx, repo, userID, *req.ContextID, examID, now)
			if err != nil {
				return err
			}
			assignment = locked
			if err := s.checkIntegrityConsent(assignment, req.IntegrityConsent); err != nil {
				return err
			}
		}

		exam, err := repo.GetQuestionsByExamID(ctx, examID)
		if err != nil {
			return err
		}

		// Đề THPT/TOEIC chưa công khai chỉ được làm qua bài tập lớp; trả "không tìm thấy" để
		// không lộ sự tồn tại của đề cho người ngoài lớp.
		if (exam.Type == model.ExamTypeTHPT || exam.Type == model.ExamTypeTOEIC) &&
			!exam.IsPublic &&
			req.ContextType != model.AttemptContextTypeClassroomAssignment {
			return gorm.ErrRecordNotFound
		}

		// Có những ràng buộc chỉ kiểm được khi đã biết loại đề (vd part/section hợp lệ).
		if err := dto.ValidateCreateExamAttemptRequestForExam(exam.Type, req); err != nil {
			return httpx.NewError(http.StatusBadRequest, err.Error(), err)
		}

		// Chọn thời gian làm bài (phút) theo chế độ:
		//   - ASSIGNMENT: dùng thời lượng giáo viên đặt cho bài tập.
		//   - TEST: dùng thời gian chuẩn của loại đề (THPT 60, TOEIC 120) để giống thi thật.
		//   - PRACTICE: dùng thời gian người dùng chọn. Không chọn thì không giới hạn.
		// duration nil nghĩa là không giới hạn, khi đó lượt làm bài không có hạn kết thúc.
		var duration *int
		if req.Mode == model.AttemptModeAssignment {
			value := assignment.Duration
			duration = &value
		} else if req.Mode == model.AttemptModeTest {
			if exam.Type != model.ExamTypeAssignment {
				value, ok := model.GetTestDurationByExamType(exam.Type)
				if !ok {
					return ErrUnsupportedExamType
				}
				duration = &value
			}
		} else if req.Duration != nil {
			duration = valuex.MapPtr(req.Duration, func(value model.AttemptDuration) int {
				return int(value)
			})
		}

		// Hạn kết thúc để worker tự nộp khi hết giờ. Bài tập lớp không được vượt quá hạn nộp
		// của assignment, kể cả khi bắt đầu sát hạn.
		var expiresAt *time.Time
		if duration != nil {
			value := now.Add(time.Duration(*duration) * time.Minute)
			if assignment != nil {
				value = assignmentAttemptExpiresAt(now, *duration, assignment.DueAt)
			}
			expiresAt = &value
		}

		// Phạm vi câu hỏi được chốt vào lượt làm bài để các bước sau (chấm điểm, chia trang,
		// xem lại) cùng dựa trên một tập câu. Luyện tập THPT không chọn phạm vi thì làm cả đề.
		var sectionCode *model.SectionCode
		partCodes := make([]model.PartCode, 0, len(req.Parts))
		if len(req.Parts) > 0 {
			partCodes = append(partCodes, req.Parts...)
		} else if req.Section != nil {
			value := *req.Section
			sectionCode = &value
		} else if exam.Type == model.ExamTypeTHPT && req.Mode == model.AttemptModePractice {
			value := model.SectionCodeFull
			sectionCode = &value
		}

		// Số câu trong phạm vi, làm mẫu số khi tính kết quả.
		totalQuestions := 0
		eachScopedQuestion(exam, sectionCode, partCodes, func(*model.Question) bool {
			totalQuestions++
			return true
		})

		attempt = &model.Attempt{
			Mode:           req.Mode,
			Status:         model.AttemptStatusInProgress,
			SectionCode:    sectionCode,
			PartCodes:      partCodes,
			StartedAt:      now,
			ExpiresAt:      expiresAt,
			SubmittedAt:    nil,
			Duration:       duration,
			TotalQuestions: totalQuestions,
			CorrectAnswers: nil,
			Score:          nil,
			ExamID:         exam.ID,
			UserID:         userID,
			ContextType:    req.ContextType,
			ContextID:      req.ContextID,
			Flow:           s.attemptFlowFor(exam, req.Mode),
			Layout:         buildAttemptLayout(exam, sectionCode, partCodes, s.listeningGap()),
		}
		// TOEIC_STRICT bắt đầu ở phần nghe nếu phạm vi có trang nghe, và tính giờ trang nghe
		// đầu tiên từ lúc tạo; không có phần nghe thì vào thẳng phần đọc.
		if attempt.Flow == model.AttemptFlowTOEICStrict {
			stage := model.AttemptStageReading
			if hasListeningPage(attempt.Layout) {
				stage = model.AttemptStageListening
				attempt.ListeningPageStartedAt = &now
			}
			attempt.Stage = &stage
		}
		s.applyIntegrityPolicy(attempt, assignment, now)

		if err := repo.CreateAttempt(ctx, attempt); err != nil {
			return err
		}
		// Gắn sẵn đề và danh sách đáp án rỗng để dựng response mà không phải đọc lại DB.
		attempt.Exam = *exam
		attempt.Answers = []model.AttemptAnswer{}

		// Bài tập lớp: tạo bài nộp cùng transaction để gradebook của giáo viên thấy học viên
		// đã bắt đầu, và không bao giờ có lượt làm bài mà thiếu bài nộp hay ngược lại.
		if attempt.ContextType == model.AttemptContextTypeClassroomAssignment && attempt.ContextID != nil {
			gradebook, err = createAssignmentSubmission(ctx, repo, assignment, userID, attempt.ID, now)
			if err != nil {
				return err
			}
		}
		return nil
	})
	// Hai request tạo cùng lúc vượt qua được bước kiểm tra trong tx sẽ vấp unique index ở
	// bước insert; đổi lỗi đó về đúng lỗi nghiệp vụ thay vì 500.
	if err != nil {
		if platformdb.IsUniqueViolation(err, model.AttemptActiveUserUniqueIndex) {
			return nil, ErrActiveAttemptExists
		}
		if platformdb.IsUniqueViolation(err, model.AttemptClassroomAssignmentUniqueIndex) {
			return nil, ErrContextAttemptAlreadyExists
		}
		return nil, httpx.FromError(err, "[ERROR] Failed to create attempt")
	}
	// Báo gradebook realtime cho giáo viên sau khi commit, để không đẩy sự kiện của một
	// transaction có thể bị rollback.
	if gradebook != nil {
		s.publishGradebookEvents([]*gradebookEvent{gradebook})
	}
	// TOEIC_STRICT không trả cả đề để client không xem trước được các trang chưa tới;
	// nội dung lấy theo từng trang qua API pages.
	if attempt.Flow == model.AttemptFlowTOEICStrict {
		return dto.ToAttemptResponse(attempt, s.clock()), nil
	}
	return dto.ToAttemptResponseWithQuestions(attempt, s.clock()), nil
}

// GetAttempts liệt kê lượt làm bài của người dùng theo trạng thái (đang làm, đã nộp, đã hủy),
// có lọc theo chế độ, loại đề, tên đề. Không kèm câu hỏi để danh sách nhẹ.
func (s *service) GetAttempts(ctx context.Context, userID string, query dto.GetAttemptsQuery) ([]*dto.AttemptResponse, int64, *httpx.HTTPError) {
	attempts, total, err := s.repo.GetAttemptsByUserIDAndStatus(ctx, userID, repository.AttemptFilter{Status: query.Status, ExcludeMode: query.ExcludeMode, ExamTypes: query.ExamTypes(), Search: query.Search}, query.Limit(), query.Offset())
	if err != nil {
		return nil, 0, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get attempts", err)
	}

	now := s.clock()
	responses := make([]*dto.AttemptResponse, len(attempts))
	for i, attempt := range attempts {
		responses[i] = dto.ToAttemptResponse(attempt, now)
	}

	return responses, total, nil
}

// GetAttemptQuestions trả toàn bộ câu hỏi trong phạm vi của lượt đang làm, kèm các đáp án đã
// lưu, để client dựng lại bài và làm tiếp sau khi tải lại trang. Chỉ dành cho luồng FREE.
func (s *service) GetAttemptQuestions(ctx context.Context, userID string, id uint) (*dto.AttemptResponse, *httpx.HTTPError) {
	attempt, err := s.repo.GetAttemptQuestionsByIDAndUserID(ctx, id, userID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get attempt questions")
	}

	// Chỉ lượt đang làm mới có câu hỏi để làm tiếp; lượt đã nộp hoặc đã hủy xem lại qua history.
	if attempt.Status != model.AttemptStatusInProgress {
		return nil, ErrAttemptNotActive
	}
	// TOEIC_STRICT không được lấy cả đề một lần: client sẽ có sẵn nội dung các trang nghe chưa
	// tới hoặc đã khóa, phá luật thi. Luồng này dựng lại bài qua navigator và lấy từng trang.
	if attempt.Flow == model.AttemptFlowTOEICStrict {
		return nil, ErrAttemptPagedFlow
	}

	// Hết giờ mà worker chưa kịp nộp: nộp luôn bằng đáp án đã lưu và báo hết giờ, để học viên
	// không làm tiếp được sau giờ bằng cách tải lại trang. Bài đã được nộp ở nơi khác thì bỏ qua.
	if attempt.ExpiresAt != nil && !s.clock().Before(*attempt.ExpiresAt) {
		if _, err := s.submitAttempt(ctx, userID, attempt.ID, nil); err != nil &&
			!errors.Is(err, ErrAttemptAlreadySubmitted) {
			return nil, httpx.FromError(err, "[ERROR] Failed to get attempt questions")
		}
		return nil, ErrAttemptExpired
	}

	return dto.ToAttemptResponseWithQuestions(attempt, s.clock()), nil
}

// CancelAttempt cho người dùng bỏ dở lượt tự làm đề: lượt chuyển sang CANCELLED, không chấm
// điểm, và vẫn lưu bản chụp để xem lại những gì đã làm. Hủy xong thì người dùng tạo được lượt
// mới, vì mỗi người chỉ có một lượt đang làm.
func (s *service) CancelAttempt(ctx context.Context, userID string, attemptID uint) (*dto.AttemptResponse, *httpx.HTTPError) {
	attempt, err := s.repo.GetAttemptForLifecycle(ctx, attemptID, userID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to cancel attempt")
	}
	// Lượt làm bài tập lớp không hủy được: mỗi bài tập chỉ được làm một lần, hủy rồi làm lại thì
	// lách được luật đó. Muốn dừng thì phải nộp. Kiểm trước khi mở transaction vì loại lượt
	// không bao giờ đổi.
	if attempt.ContextType == model.AttemptContextTypeClassroomAssignment {
		return nil, ErrAssignmentAttemptCancel
	}

	var response *dto.AttemptResponse
	err = s.repo.Transaction(ctx, func(repo repository.Repository) error {
		// Đọc lại có khóa dòng trong tx, để hủy không chạy chen với lúc bài đang được nộp (tay
		// hoặc worker hết giờ): hai bên chờ nhau và bên sau thấy trạng thái mới.
		attempt, err = repo.GetAttemptForLifecycle(ctx, attemptID, userID)
		if err != nil {
			return err
		}

		// Đã hủy rồi: trả lại bản chụp cũ (bấm hủy hai lần không lỗi). Đã nộp: không hủy được
		// nữa, kết quả đã chốt.
		switch attempt.Status {
		case model.AttemptStatusCancelled:
			if attempt.History == nil {
				return ErrAttemptHistoryNotFound
			}
			response, err = dto.UnmarshalAttemptHistory(attempt.History.Data)
			return err
		case model.AttemptStatusSubmitted:
			return ErrAttemptAlreadySubmitted
		case model.AttemptStatusInProgress:
		default:
			return ErrAttemptNotActive
		}
		// Chuyển sang CANCELLED và xóa mọi dấu vết chấm điểm, để lượt hủy không được tính vào
		// thống kê điểm như một bài đã nộp.
		attempt.Status = model.AttemptStatusCancelled
		attempt.SubmittedAt = nil
		attempt.CorrectAnswers = nil
		attempt.Score = nil
		if err := repo.UpdateAttempt(ctx, attempt); err != nil {
			return err
		}

		// Lưu bản chụp để người dùng xem lại các câu đã làm. Bài hủy không kèm kết quả chấm
		// (ToAttemptHistoryResponse chỉ kèm kết quả khi bài đã nộp).
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
		response = dto.ToAttemptHistoryResponse(attempt)
		return nil
	})
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to cancel attempt")
	}

	return response, nil
}

// GetAttemptHistory trả bản chụp để xem lại lượt đã nộp hoặc đã hủy, kèm thông tin lớp và
// ghi chú của giáo viên (nếu là bài tập lớp).
func (s *service) GetAttemptHistory(ctx context.Context, userID string, attemptID uint) (*dto.AttemptResponse, *httpx.HTTPError) {
	history, err := s.repo.GetAttemptHistoryByIDAndUserID(ctx, attemptID, userID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get attempt history")
	}

	response, err := dto.UnmarshalAttemptHistory(history.Data)
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to read attempt history", err)
	}
	s.attachClassroom(ctx, response)
	s.attachTeacherNote(ctx, userID, response)
	return response, nil
}

// attachTeacherNote đọc lời nhắn từ attempt hiện tại vì history là bản chụp lúc nộp,
// còn đình chỉ / mở lại có thể xảy ra sau đó. Lỗi thì bỏ qua: banner vẫn hiện, chỉ thiếu lời nhắn.
func (s *service) attachTeacherNote(ctx context.Context, userID string, response *dto.AttemptResponse) {
	if response == nil || !response.Proctored {
		return
	}
	note, err := s.repo.GetAttemptTeacherNote(ctx, response.ID, userID)
	if err != nil || note == nil || strings.TrimSpace(*note) == "" {
		return
	}
	response.TeacherNote = note
}

// attachClassroom điền ClassroomID cho bài giao để client mở kết quả trong lớp.
// Không tìm được lớp thì bỏ qua: client quay về trang lịch sử chung.
func (s *service) attachClassroom(ctx context.Context, response *dto.AttemptResponse) {
	if response == nil || response.AssignmentID == nil || response.ClassroomID != nil {
		return
	}
	assignment, err := s.repo.GetAssignmentForAttemptValidation(ctx, *response.AssignmentID)
	if err != nil {
		return
	}
	response.ClassroomID = &assignment.ClassroomID
}
