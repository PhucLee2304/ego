package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"net/http"
	"strings"

	identityClient "ego/api/gen/go/identity"
	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
)

func (s *service) CreateClassroom(ctx context.Context, teacherID string, req dto.CreateClassroomRequest) (*dto.CreatedClassroomResponse, *httpx.HTTPError) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const codeLength = 8
	const maxCodeAttempts = 5

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
		// Chỉ ghi nhận lớp khi cả dữ liệu lớp và thông tin giáo viên dùng cho response đều lấy thành công.
		err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
			classroom := &model.Classroom{
				Code:        string(codeBytes),
				Name:        strings.TrimSpace(req.Name),
				Description: req.Description,
				Avatar:      req.Avatar,
				TeacherID:   teacherID,
				Active:      true,
			}
			if err := repo.CreateClassroom(ctx, classroom); err != nil {
				return err
			}

			response = dto.ToCreatedClassroomResponse(classroom)
			// Lấy thông tin hiển thị của giáo viên từ Users thay vì sao chép dữ liệu người dùng sang Classrooms.
			usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: []string{teacherID}})
			if err != nil {
				return err
			}
			usersByID := dto.ToUserInfoMap(usersResp.Users)
			response.Teacher = usersByID[teacherID]
			return nil
		})
		if err == nil {
			return response, nil
		}
		// Chỉ sinh mã mới khi vi phạm unique; các lỗi khác phải được trả về ngay.
		if !platformdb.IsUniqueViolation(err, "") {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
	}
	return nil, httpx.NewError(http.StatusInternalServerError, errClassroomServiceFailed, errors.New("[ERROR] Could not generate a unique classroom code"))
}

func (s *service) GetClassrooms(ctx context.Context, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int, *httpx.HTTPError) {
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
	return responses, httpx.ToPageCounts(total, query.PageSize), nil
}

func (s *service) GetMyClassrooms(ctx context.Context, userID string, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int, *httpx.HTTPError) {
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
	return responses, httpx.ToPageCounts(total, query.PageSize), nil
}

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
	usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: userIDs})
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	usersByID := dto.ToUserInfoMap(usersResp.Users)
	response.Teacher = usersByID[classroom.TeacherID]
	for i := range classroom.Members {
		response.Members[i].User = usersByID[classroom.Members[i].UserID]
	}
	return response, nil
}

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
	usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: []string{classroom.TeacherID}})
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	usersByID := dto.ToUserInfoMap(usersResp.Users)
	response.Teacher = usersByID[classroom.TeacherID]
	return response, nil
}

func (s *service) UpdateClassroom(ctx context.Context, teacherID string, id uint, req dto.UpdateClassroomRequest) (*dto.ClassroomResponse, *httpx.HTTPError) {
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
		if req.Avatar != nil {
			classroom.Avatar = req.Avatar
		}
		if err := repo.UpdateClassroom(ctx, classroom); err != nil {
			return err
		}
		response = dto.ToClassroomResponse(classroom)
		usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: []string{classroom.TeacherID}})
		if err != nil {
			return err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		response.Teacher = usersByID[classroom.TeacherID]
		return nil
	}); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	return response, nil
}

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
		usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: []string{classroom.TeacherID}})
		if err != nil {
			return err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		response.Teacher = usersByID[classroom.TeacherID]
		return nil
	}); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	return response, nil
}
