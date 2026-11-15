package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"net/http"
	"strings"

	identityClient "ego/api/gen/go/identity"
	storageClient "ego/api/gen/go/storage"
	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/platform/logger"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"

	"gorm.io/gorm"
)

// CreateClassroom tạo lớp mới cho giáo viên với mã lớp ngẫu nhiên 8 ký tự để học sinh nhập
// khi xin vào lớp. Mã trùng thì sinh lại, tối đa vài lần.
func (s *service) CreateClassroom(ctx context.Context, teacherID string, req dto.CreateClassroomRequest) (*dto.CreatedClassroomResponse, *httpx.HTTPError) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const codeLength = 8
	const maxCodeAttempts = 5

	avatar, httpErr := s.resolveClassroomAvatar(ctx, req.Avatar)
	if httpErr != nil {
		return nil, httpErr
	}

	// Thử sinh mã lớp tối đa một số lần hữu hạn để xử lý trường hợp mã ngẫu nhiên bị trùng.
	for codeAttempt := 0; codeAttempt < maxCodeAttempts; codeAttempt++ {
		// Dùng nguồn ngẫu nhiên mật mã và loại các ký tự dễ nhầm lẫn khi học sinh nhập mã lớp.
		codeBytes := make([]byte, codeLength)
		for i := range codeBytes {
			index, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
			if err != nil {
				return nil, httpx.FromError(err, errClassroomServiceFailed)
			}
			codeBytes[i] = alphabet[index.Int64()]
		}

		var response *dto.CreatedClassroomResponse
		err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
			classroom := &model.Classroom{
				Code:        string(codeBytes),
				Name:        strings.TrimSpace(req.Name),
				Description: req.Description,
				Avatar:      avatar,
				TeacherID:   teacherID,
				Active:      true,
			}
			if err := repo.CreateClassroom(ctx, classroom); err != nil {
				return err
			}

			response = dto.ToCreatedClassroomResponse(classroom)
			return nil
		})
		if err == nil {
			// Lấy thông tin hiển thị của giáo viên từ Users sau khi commit (không gọi gRPC trong tx).
			response.Teacher = s.usersInfo(ctx, teacherID)[teacherID]
			return response, nil
		}
		// Chỉ sinh mã mới khi vi phạm unique; các lỗi khác phải được trả về ngay.
		if !platformdb.IsUniqueViolation(err, "") {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
	}
	return nil, httpx.NewError(http.StatusInternalServerError, errClassroomServiceFailed, errors.New("[ERROR] Could not generate a unique classroom code"))
}

// GetClassrooms (admin) liệt kê mọi lớp có phân trang. Tìm theo tên giáo viên được đổi sang
// tìm theo danh sách teacher ID qua identity, vì tên người dùng không lưu ở learning.
func (s *service) GetClassrooms(ctx context.Context, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int64, *httpx.HTTPError) {
	var teacherIDs []string
	if query.Query != "" {
		usersResp, err := s.identityClient.SearchUserIDsByName(ctx, &identityClient.SearchUserIDsByNameRequest{Query: query.Query})
		if err != nil {
			return nil, 0, httpx.FromError(err, errClassroomServiceFailed)
		}
		teacherIDs = usersResp.Ids
	}
	classrooms, total, err := s.repo.GetClassrooms(ctx, query, teacherIDs)
	if err != nil {
		return nil, 0, httpx.FromError(err, errClassroomServiceFailed)
	}

	userIDs := make([]string, 0, len(classrooms))
	seenUserIDs := make(map[string]struct{}, len(classrooms))
	for _, classroom := range classrooms {
		if _, ok := seenUserIDs[classroom.TeacherID]; !ok {
			seenUserIDs[classroom.TeacherID] = struct{}{}
			userIDs = append(userIDs, classroom.TeacherID)
		}
		for _, member := range classroom.Members {
			if _, ok := seenUserIDs[member.UserID]; !ok {
				seenUserIDs[member.UserID] = struct{}{}
				userIDs = append(userIDs, member.UserID)
			}
		}
	}
	var users []*identityClient.UserInfo
	if len(userIDs) > 0 {
		usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: userIDs})
		if err != nil {
			return nil, 0, httpx.FromError(err, errClassroomServiceFailed)
		}
		users = usersResp.Users
	}
	responses := dto.ToClassroomResponses(classrooms, users)
	return responses, total, nil
}

// GetMyClassrooms liệt kê các lớp người dùng dạy hoặc đã được duyệt vào học, kèm thông tin
// giáo viên và thành viên lấy từ identity.
func (s *service) GetMyClassrooms(ctx context.Context, userID string, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int64, *httpx.HTTPError) {
	var teacherIDs []string
	if query.Query != "" {
		usersResp, err := s.identityClient.SearchUserIDsByName(ctx, &identityClient.SearchUserIDsByNameRequest{Query: query.Query})
		if err != nil {
			return nil, 0, httpx.FromError(err, errClassroomServiceFailed)
		}
		teacherIDs = usersResp.Ids
	}
	classrooms, total, err := s.repo.GetClassroomsByUser(ctx, userID, query, teacherIDs)
	if err != nil {
		return nil, 0, httpx.FromError(err, errClassroomServiceFailed)
	}

	userIDs := make([]string, 0, len(classrooms))
	seenUserIDs := make(map[string]struct{}, len(classrooms))
	for _, classroom := range classrooms {
		if _, ok := seenUserIDs[classroom.TeacherID]; !ok {
			seenUserIDs[classroom.TeacherID] = struct{}{}
			userIDs = append(userIDs, classroom.TeacherID)
		}
		for _, member := range classroom.Members {
			if _, ok := seenUserIDs[member.UserID]; !ok {
				seenUserIDs[member.UserID] = struct{}{}
				userIDs = append(userIDs, member.UserID)
			}
		}
	}
	var users []*identityClient.UserInfo
	if len(userIDs) > 0 {
		usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: userIDs})
		if err != nil {
			return nil, 0, httpx.FromError(err, errClassroomServiceFailed)
		}
		users = usersResp.Users
	}
	responses := dto.ToClassroomResponses(classrooms, users)
	return responses, total, nil
}

// GetClassroom trả chi tiết lớp cho giáo viên hoặc học sinh đã được duyệt; người ngoài lớp
// bị từ chối để không lộ danh sách thành viên.
func (s *service) GetClassroom(ctx context.Context, userID string, id uint) (*dto.ClassroomResponse, *httpx.HTTPError) {
	classroom, err := s.repo.GetClassroomByID(ctx, id)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if classroom.TeacherID != userID {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, id, userID)
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		if !approved {
			return nil, ErrClassroomForbidden
		}
	}
	response := dto.ToClassroomResponse(classroom)
	userIDs := []string{classroom.TeacherID}
	for _, member := range classroom.Members {
		userIDs = append(userIDs, member.UserID)
	}
	usersByID := s.usersInfo(ctx, userIDs...)
	response.Teacher = usersByID[classroom.TeacherID]
	for i := range classroom.Members {
		response.Members[i].User = usersByID[classroom.Members[i].UserID]
	}
	return response, nil
}

// GetClassroomByCode tra lớp theo mã để học sinh xem trước lớp sẽ xin vào (tên, giáo viên,
// sĩ số). Lớp đã lưu trữ không nhận học sinh mới nên báo lỗi.
func (s *service) GetClassroomByCode(ctx context.Context, code string) (*dto.ClassroomByCodeResponse, *httpx.HTTPError) {
	classroom, err := s.repo.GetClassroomByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if !classroom.Active {
		return nil, ErrClassroomNotActive
	}
	countApprovedMembers, err := s.repo.CountApprovedMembers(ctx, classroom.ID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	response := dto.ToClassroomByCodeResponse(classroom, countApprovedMembers)
	response.Teacher = s.usersInfo(ctx, classroom.TeacherID)[classroom.TeacherID]
	return response, nil
}

// UpdateClassroom cho giáo viên sửa tên, mô tả, ảnh đại diện của lớp; chỉ sửa field được gửi.
// Lớp đã lưu trữ thì không sửa được.
func (s *service) UpdateClassroom(ctx context.Context, teacherID string, id uint, req dto.UpdateClassroomRequest) (*dto.ClassroomResponse, *httpx.HTTPError) {
	avatar, httpErr := s.resolveClassroomAvatar(ctx, req.Avatar)
	if httpErr != nil {
		return nil, httpErr
	}
	var response *dto.ClassroomResponse
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, id)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		if !classroom.Active {
			return ErrClassroomNotActive
		}
		if req.Name != nil {
			classroom.Name = strings.TrimSpace(*req.Name)
		}
		if req.Description != nil {
			classroom.Description = req.Description
		}
		if avatar != nil {
			classroom.Avatar = avatar
		}
		if err := repo.UpdateClassroom(ctx, classroom); err != nil {
			return err
		}
		response = dto.ToClassroomResponse(classroom)
		return nil
	}); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	response.Teacher = s.usersInfo(ctx, teacherID)[teacherID]
	return response, nil
}

// ArchiveClassroom lưu trữ lớp (Active = false) thay vì xóa, để giữ lịch sử bài tập và điểm.
// Không lưu trữ được khi đang có buổi học trực tuyến diễn ra.
func (s *service) ArchiveClassroom(ctx context.Context, teacherID string, id uint) (*dto.ClassroomResponse, *httpx.HTTPError) {
	var response *dto.ClassroomResponse
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, id)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		live, err := repo.HasLiveRoomByClassroom(ctx, id)
		if err != nil {
			return err
		}
		if live {
			return ErrRoomLive
		}
		classroom.Active = false
		if err := repo.UpdateClassroom(ctx, classroom); err != nil {
			return err
		}
		response = dto.ToClassroomResponse(classroom)
		return nil
	}); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	response.Teacher = s.usersInfo(ctx, teacherID)[teacherID]
	return response, nil
}

// GetClassroomPostAccess trả lớp (kèm member đã duyệt) và cờ userID là học sinh
// đã được duyệt; dùng cho rpc của posts.
func (s *service) GetClassroomPostAccess(ctx context.Context, classroomID uint, userID string) (*model.Classroom, bool, error) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, ErrClassroomNotFound
		}
		return nil, false, err
	}
	isApproved := false
	if classroom.TeacherID != userID && userID != "" {
		isApproved, err = s.repo.IsStudentApprovedInClassroom(ctx, classroom.ID, userID)
		if err != nil {
			return nil, false, err
		}
	}
	return classroom, isApproved, nil
}

// resolveClassroomAvatar đổi tên file đã upload vào avatars/classrooms thành URL công khai, giống avatar user ở identity.
func (s *service) resolveClassroomAvatar(ctx context.Context, fileName *string) (*string, *httpx.HTTPError) {
	if fileName == nil {
		return nil, nil
	}
	if name := *fileName; name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return nil, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid avatar", nil)
	}
	resp, err := s.storageClient.GetPublicURL(ctx, &storageClient.GetPublicURLRequest{
		Folders:  []string{"avatars", "classrooms"},
		FileName: *fileName,
	})
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get avatar URL", err)
	}
	return &resp.Url, nil
}

// usersInfo lấy thông tin hiển thị của các user từ identity (gọi ngoài transaction).
// Mọi id đều có mặt trong map: user identity không trả về hoặc identity lỗi thì chỉ có ID,
// để client không gặp teacher/user null. Lỗi identity chỉ ghi log, response thiếu tên/ảnh.
func (s *service) usersInfo(ctx context.Context, ids ...string) map[string]*dto.UserInfo {
	unique := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok || id == "" {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	usersByID := map[string]*dto.UserInfo{}
	if len(unique) > 0 && s.identityClient != nil {
		resp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: unique})
		if err != nil {
			logger.Log.Error().Err(err).Strs("user_ids", unique).Msg("[ERROR] Failed to load users info")
		} else {
			usersByID = dto.ToUserInfoMap(resp.Users)
		}
	}
	for _, id := range unique {
		if usersByID[id] == nil {
			usersByID[id] = &dto.UserInfo{ID: id}
		}
	}
	return usersByID
}
