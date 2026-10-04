package service

import (
	"context"
	"crypto/rand"
	usersClient "ego/api/gen/go/users"
	"ego/platform/httpx"
	classroomsConfig "ego/services/classrooms/config"
	"ego/services/classrooms/internal/dto"
	"ego/services/classrooms/internal/model"
	"ego/services/classrooms/internal/repository"
	"errors"
	"strings"
	"time"
)

type Service interface {
	CreateClassroom(ctx context.Context, teacherID string, req dto.CreateClassroomRequest) (*dto.CreatedClassroomResponse, error)
	GetClassrooms(ctx context.Context, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int, error)
	GetMyClassrooms(ctx context.Context, userID string, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int, error)
	GetClassroom(ctx context.Context, userID string, id uint) (*dto.ClassroomResponse, error)
	GetClassroomByCode(ctx context.Context, code string) (*dto.ClassroomByCodeResponse, error)
	UpdateClassroom(ctx context.Context, teacherID string, id uint, req dto.UpdateClassroomRequest) (*dto.ClassroomResponse, error)
	ArchiveClassroom(ctx context.Context, teacherID string, id uint) (*dto.ClassroomResponse, error)
	RequestJoinClassroom(ctx context.Context, userID string, req dto.RequestJoinClassroomRequest) (*dto.ClassroomJoinRequestResponse, error)
	GetJoinRequests(ctx context.Context, teacherID string, classroomID uint) ([]*dto.ClassroomJoinRequestResponse, error)
	ApproveMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, error)
	RejectMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, error)
	RemoveMember(ctx context.Context, teacherID string, classroomID uint, userID string) error
}

type service struct {
	repo        *repository.Repository
	cfg         *classroomsConfig.AppConfig
	usersClient usersClient.UserServiceClient
}

func New(repo *repository.Repository, appConfig *classroomsConfig.AppConfig, usersServiceClient usersClient.UserServiceClient) Service {
	return &service{
		repo:        repo,
		cfg:         appConfig,
		usersClient: usersServiceClient,
	}
}

var (
	ErrClassroomNotActive     = errors.New("[ERROR] Classroom is not active")
	ErrClassroomForbidden     = errors.New("[FORBIDDEN] Classroom access denied")
	ErrClassroomCapacityFull  = errors.New("[ERROR] Classroom capacity is full")
	ErrTeacherCannotJoinClass = errors.New("[ERROR] Teacher cannot join own classroom as student")
)

func (s *service) CreateClassroom(ctx context.Context, teacherID string, req dto.CreateClassroomRequest) (*dto.CreatedClassroomResponse, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const codeLength = 8
	codeBytes := make([]byte, codeLength)
	if _, err := rand.Read(codeBytes); err != nil {
		return nil, err
	}
	for i, value := range codeBytes {
		codeBytes[i] = alphabet[int(value)%len(alphabet)]
	}

	var response *dto.CreatedClassroomResponse
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
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
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: []string{teacherID}})
		if err != nil {
			return err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		response.Teacher = usersByID[teacherID]
		return nil
	}); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *service) GetClassrooms(ctx context.Context, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int, error) {
	classrooms, total, err := s.repo.GetClassrooms(ctx, query.PaginationQuery, query.Query)
	if err != nil {
		return nil, 0, err
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
	var users []*usersClient.UserInfo
	if len(userIDs) > 0 {
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: userIDs})
		if err != nil {
			return nil, 0, err
		}
		users = usersResp.Users
	}
	responses := dto.ToClassroomResponses(classrooms, users)
	return responses, httpx.ToPageCounts(total, query.PageSize), nil
}

func (s *service) GetMyClassrooms(ctx context.Context, userID string, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int, error) {
	classrooms, total, err := s.repo.GetClassroomsByUser(ctx, userID, query.PaginationQuery, query.Query)
	if err != nil {
		return nil, 0, err
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
	var users []*usersClient.UserInfo
	if len(userIDs) > 0 {
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: userIDs})
		if err != nil {
			return nil, 0, err
		}
		users = usersResp.Users
	}
	responses := dto.ToClassroomResponses(classrooms, users)
	return responses, httpx.ToPageCounts(total, query.PageSize), nil
}

func (s *service) GetClassroom(ctx context.Context, userID string, id uint) (*dto.ClassroomResponse, error) {
	classroom, err := s.repo.GetClassroomByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if classroom.TeacherID != userID {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, id, userID)
		if err != nil {
			return nil, err
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
	usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: userIDs})
	if err != nil {
		return nil, err
	}
	usersByID := dto.ToUserInfoMap(usersResp.Users)
	response.Teacher = usersByID[classroom.TeacherID]
	for i := range classroom.Members {
		response.Members[i].User = usersByID[classroom.Members[i].UserID]
	}
	return response, nil
}

func (s *service) GetClassroomByCode(ctx context.Context, code string) (*dto.ClassroomByCodeResponse, error) {
	classroom, err := s.repo.GetClassroomByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return nil, err
	}
	if !classroom.Active {
		return nil, ErrClassroomNotActive
	}
	countApprovedMembers, err := s.repo.CountApprovedMembers(ctx, classroom.ID)
	if err != nil {
		return nil, err
	}
	response := dto.ToClassroomByCodeResponse(classroom, countApprovedMembers)
	usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: []string{classroom.TeacherID}})
	if err != nil {
		return nil, err
	}
	usersByID := dto.ToUserInfoMap(usersResp.Users)
	response.Teacher = usersByID[classroom.TeacherID]
	return response, nil
}

func (s *service) UpdateClassroom(ctx context.Context, teacherID string, id uint, req dto.UpdateClassroomRequest) (*dto.ClassroomResponse, error) {
	var response *dto.ClassroomResponse
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
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
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: []string{classroom.TeacherID}})
		if err != nil {
			return err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		response.Teacher = usersByID[classroom.TeacherID]
		return nil
	}); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *service) ArchiveClassroom(ctx context.Context, teacherID string, id uint) (*dto.ClassroomResponse, error) {
	var response *dto.ClassroomResponse
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, id)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		classroom.Active = false
		if err := repo.UpdateClassroom(ctx, classroom); err != nil {
			return err
		}
		response = dto.ToClassroomResponse(classroom)
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: []string{classroom.TeacherID}})
		if err != nil {
			return err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		response.Teacher = usersByID[classroom.TeacherID]
		return nil
	}); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *service) RequestJoinClassroom(ctx context.Context, userID string, req dto.RequestJoinClassroomRequest) (*dto.ClassroomJoinRequestResponse, error) {
	var response *dto.ClassroomJoinRequestResponse
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		classroom, err := repo.GetClassroomByCode(ctx, strings.ToUpper(strings.TrimSpace(req.Code)))
		if err != nil {
			return err
		}
		if !classroom.Active {
			return ErrClassroomNotActive
		}
		if classroom.TeacherID == userID {
			return ErrTeacherCannotJoinClass
		}
		count, err := repo.CountApprovedMembers(ctx, classroom.ID)
		if err != nil {
			return err
		}
		if count >= int64(s.cfg.MaxStudentsPerClass) {
			return ErrClassroomCapacityFull
		}

		member := &model.Member{
			ClassroomID: classroom.ID,
			UserID:      userID,
			Approved:    nil,
			ApprovedAt:  nil,
			JoinedAt:    nil,
		}
		if err := repo.UpsertMember(ctx, member); err != nil {
			return err
		}
		member.Classroom = *classroom
		response = dto.ToClassroomJoinRequestResponse(member)
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: []string{member.UserID, classroom.TeacherID}})
		if err != nil {
			return err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		response.User = usersByID[member.UserID]
		response.Classroom.Teacher = usersByID[classroom.TeacherID]
		return nil
	}); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *service) GetJoinRequests(ctx context.Context, teacherID string, classroomID uint) ([]*dto.ClassroomJoinRequestResponse, error) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	members, err := s.repo.GetJoinRequests(ctx, classroomID)
	if err != nil {
		return nil, err
	}

	responses := make([]*dto.ClassroomJoinRequestResponse, len(members))
	userIDs := make([]string, 0, len(members)*2)
	seenUserIDs := make(map[string]struct{}, len(members)*2)
	for i, member := range members {
		responses[i] = dto.ToClassroomJoinRequestResponse(member)
		if _, ok := seenUserIDs[member.UserID]; !ok {
			seenUserIDs[member.UserID] = struct{}{}
			userIDs = append(userIDs, member.UserID)
		}
		if member.Classroom.TeacherID != "" {
			if _, ok := seenUserIDs[member.Classroom.TeacherID]; !ok {
				seenUserIDs[member.Classroom.TeacherID] = struct{}{}
				userIDs = append(userIDs, member.Classroom.TeacherID)
			}
		}
	}
	if len(userIDs) > 0 {
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: userIDs})
		if err != nil {
			return nil, err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		for i, response := range responses {
			response.User = usersByID[members[i].UserID]
			if response.Classroom != nil {
				response.Classroom.Teacher = usersByID[members[i].Classroom.TeacherID]
			}
		}
	}

	return responses, nil
}

func (s *service) ApproveMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, error) {
	var response *dto.ClassroomJoinRequestResponse
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, classroomID)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		if !classroom.Active {
			return ErrClassroomNotActive
		}
		if classroom.TeacherID == userID {
			return ErrTeacherCannotJoinClass
		}
		member, err := repo.GetMember(ctx, classroomID, userID)
		if err != nil {
			return err
		}
		approved := true
		now := time.Now()
		member.Approved = &approved
		member.ApprovedAt = &now
		member.JoinedAt = &now
		if err := repo.UpsertMember(ctx, member); err != nil {
			return err
		}
		response = dto.ToClassroomJoinRequestResponse(member)
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: []string{member.UserID, member.Classroom.TeacherID}})
		if err != nil {
			return err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		response.User = usersByID[member.UserID]
		response.Classroom.Teacher = usersByID[member.Classroom.TeacherID]
		return nil
	}); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *service) RejectMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, error) {
	var response *dto.ClassroomJoinRequestResponse
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, classroomID)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		if !classroom.Active {
			return ErrClassroomNotActive
		}
		member, err := repo.GetMember(ctx, classroomID, userID)
		if err != nil {
			return err
		}
		approved := false
		now := time.Now()
		member.Approved = &approved
		member.ApprovedAt = &now
		member.JoinedAt = nil
		if err := repo.UpsertMember(ctx, member); err != nil {
			return err
		}
		response = dto.ToClassroomJoinRequestResponse(member)
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: []string{member.UserID, member.Classroom.TeacherID}})
		if err != nil {
			return err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		response.User = usersByID[member.UserID]
		response.Classroom.Teacher = usersByID[member.Classroom.TeacherID]
		return nil
	}); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *service) RemoveMember(ctx context.Context, teacherID string, classroomID uint, userID string) error {
	return s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, classroomID)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		member, err := repo.GetMember(ctx, classroomID, userID)
		if err != nil {
			return err
		}
		return repo.DeleteMember(ctx, member)
	})
}
