package rpc

import (
	"context"
	"errors"

	learningClient "ego/api/gen/go/learning"
	platformRpc "ego/platform/rpc"
	"ego/services/learning/internal/service"

	"google.golang.org/grpc/codes"
)

func (s *server) GetClassroomPostAccess(ctx context.Context, req *learningClient.GetClassroomPostAccessRequest) (*learningClient.GetClassroomPostAccessResponse, error) {
	classroom, isApproved, err := s.service.GetClassroomPostAccess(ctx, uint(req.ClassroomId), req.UserId)
	if err != nil {
		if errors.Is(err, service.ErrClassroomNotFound) {
			return nil, platformRpc.PublicError(codes.NotFound, "[ERROR] Classroom not found")
		}
		return nil, err
	}

	studentIDs := make([]string, 0, len(classroom.Members))
	for _, member := range classroom.Members {
		if member.UserID != classroom.TeacherID {
			studentIDs = append(studentIDs, member.UserID)
		}
	}

	return &learningClient.GetClassroomPostAccessResponse{
		ClassroomId:        uint64(classroom.ID),
		ClassroomName:      classroom.Name,
		TeacherId:          classroom.TeacherID,
		Active:             classroom.Active,
		IsTeacher:          classroom.TeacherID == req.UserId,
		IsApprovedMember:   isApproved,
		ApprovedStudentIds: studentIDs,
	}, nil
}
