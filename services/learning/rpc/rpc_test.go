package rpc

import (
	"context"
	"errors"
	"testing"

	learningClient "ego/api/gen/go/learning"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type fakeService struct {
	service.Service
	classroom  *model.Classroom
	isApproved bool
	err        error
}

func (f *fakeService) GetClassroomPostAccess(context.Context, uint, string) (*model.Classroom, bool, error) {
	return f.classroom, f.isApproved, f.err
}

func TestGetClassroomPostAccess(t *testing.T) {
	classroom := &model.Classroom{Model: gorm.Model{ID: 7}, Name: "Math", TeacherID: "teacher", Active: true,
		Members: []model.Member{{UserID: "teacher"}, {UserID: "student-1"}, {UserID: "student-2"}}}
	boom := errors.New("boom")

	tests := []struct {
		name    string
		svc     *fakeService
		userID  string
		code    codes.Code
		wantErr error
		check   func(t *testing.T, resp *learningClient.GetClassroomPostAccessResponse)
	}{
		{
			name: "not found", svc: &fakeService{err: service.ErrClassroomNotFound}, code: codes.NotFound,
		},
		{
			name: "other error is returned as is", svc: &fakeService{err: boom}, wantErr: boom,
		},
		{
			name: "teacher", svc: &fakeService{classroom: classroom}, userID: "teacher",
			check: func(t *testing.T, resp *learningClient.GetClassroomPostAccessResponse) {
				if !resp.IsTeacher || resp.IsApprovedMember || resp.ClassroomId != 7 || resp.ClassroomName != "Math" || !resp.Active {
					t.Fatalf("unexpected response: %#v", resp)
				}
				if len(resp.ApprovedStudentIds) != 2 || resp.ApprovedStudentIds[0] != "student-1" {
					t.Fatalf("teacher must be excluded from students: %#v", resp.ApprovedStudentIds)
				}
			},
		},
		{
			name: "approved student", svc: &fakeService{classroom: classroom, isApproved: true}, userID: "student-1",
			check: func(t *testing.T, resp *learningClient.GetClassroomPostAccessResponse) {
				if resp.IsTeacher || !resp.IsApprovedMember {
					t.Fatalf("unexpected response: %#v", resp)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := New(tt.svc).GetClassroomPostAccess(context.Background(), &learningClient.GetClassroomPostAccessRequest{ClassroomId: 7, UserId: tt.userID})
			switch {
			case tt.code != codes.OK:
				if got := status.Code(err); got != tt.code {
					t.Fatalf("code = %s, want %s", got, tt.code)
				}
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
				tt.check(t, resp)
			}
		})
	}
}
