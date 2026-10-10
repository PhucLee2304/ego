package integration_test

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"testing"

	examsClient "ego/api/gen/go/exams"
	usersClient "ego/api/gen/go/users"
	"ego/services/classrooms/config"
	"ego/services/classrooms/database"
	"ego/services/classrooms/internal/dto"
	"ego/services/classrooms/internal/repository"
	"ego/services/classrooms/internal/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type testUsersServer struct {
	usersClient.UnimplementedUserServiceServer
}

func (testUsersServer) GetUsersInfo(_ context.Context, req *usersClient.GetUsersInfoRequest) (*usersClient.GetUsersInfoResponse, error) {
	users := make([]*usersClient.UserInfo, len(req.Ids))
	for i, id := range req.Ids {
		users[i] = &usersClient.UserInfo{Id: id, Email: id + "@example.com", Name: id}
	}
	return &usersClient.GetUsersInfoResponse{Users: users}, nil
}

func (testUsersServer) SearchUserIDsByName(_ context.Context, req *usersClient.SearchUserIDsByNameRequest) (*usersClient.SearchUserIDsByNameResponse, error) {
	if strings.Contains(strings.ToLower(req.Query), "teacher") {
		return &usersClient.SearchUserIDsByNameResponse{Ids: []string{"teacher-1"}}, nil
	}
	return &usersClient.SearchUserIDsByNameResponse{}, nil
}

type testExamsServer struct {
	examsClient.UnimplementedExamServiceServer
}

func (testExamsServer) SubmitStudentAssignmentAttempts(context.Context, *examsClient.SubmitStudentAssignmentAttemptsRequest) (*examsClient.SubmitStudentAssignmentAttemptsResponse, error) {
	return &examsClient.SubmitStudentAssignmentAttemptsResponse{}, nil
}

func (testExamsServer) GetAssignmentQuestionAnalytics(context.Context, *examsClient.GetAssignmentQuestionAnalyticsRequest) (*examsClient.GetAssignmentQuestionAnalyticsResponse, error) {
	return &examsClient.GetAssignmentQuestionAnalyticsResponse{Questions: []*examsClient.AssignmentQuestionAnalytics{{QuestionId: 1, Content: "Question", Order: 1, AnsweredCount: 1, WrongCount: 1}}}, nil
}

func classroomTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("CLASSROOMS_TEST_DSN")
	if dsn == "" {
		t.Skip("CLASSROOMS_TEST_DSN is not configured")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func classroomTestClients(t *testing.T) (usersClient.UserServiceClient, examsClient.ExamServiceClient) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	usersClient.RegisterUserServiceServer(server, testUsersServer{})
	examsClient.RegisterExamServiceServer(server, testExamsServer{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	conn, err := grpc.NewClient("passthrough:///classrooms-integration", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return usersClient.NewUserServiceClient(conn), examsClient.NewExamServiceClient(conn)
}

func TestClassroomMembershipLifecycleAndFilters(t *testing.T) {
	db := classroomTestDB(t)
	users, exams := classroomTestClients(t)
	cfg := &config.AppConfig{MaxStudentsPerClass: 50}
	svc := service.New(repository.NewRepository(db), cfg, users, exams)
	ctx := context.Background()

	created, err := svc.CreateClassroom(ctx, "teacher-1", dto.CreateClassroomRequest{Name: "  Advanced Math  "})
	if err != nil || created.ID == 0 || created.Name != "Advanced Math" || !created.Active || created.Code == "" {
		t.Fatalf("create classroom: response=%#v err=%v", created, err)
	}
	if _, err := svc.RequestJoinClassroom(ctx, "teacher-1", dto.RequestJoinClassroomRequest{Code: created.Code}); !errors.Is(err, service.ErrTeacherCannotJoinClass) {
		t.Fatalf("teacher joining own classroom: %v", err)
	}

	pending, err := svc.RequestJoinClassroom(ctx, "student-1", dto.RequestJoinClassroomRequest{Code: created.Code})
	if err != nil || pending.Approved != nil {
		t.Fatalf("request join: response=%#v err=%v", pending, err)
	}
	requests, err := svc.GetJoinRequests(ctx, "teacher-1", created.ID)
	if err != nil || len(requests) != 1 || requests[0].User == nil || requests[0].User.ID != "student-1" {
		t.Fatalf("get join requests: response=%#v err=%v", requests, err)
	}

	rejected, err := svc.RejectMember(ctx, "teacher-1", created.ID, "student-1")
	if err != nil || rejected.Approved == nil || *rejected.Approved {
		t.Fatalf("reject member: response=%#v err=%v", rejected, err)
	}
	if _, err := svc.RequestJoinClassroom(ctx, "student-1", dto.RequestJoinClassroomRequest{Code: created.Code}); err != nil {
		t.Fatalf("request again after rejection: %v", err)
	}
	if _, err := svc.ApproveMember(ctx, "teacher-2", created.ID, "student-1"); !errors.Is(err, service.ErrClassroomForbidden) {
		t.Fatalf("another teacher approving member: %v", err)
	}
	approved, err := svc.ApproveMember(ctx, "teacher-1", created.ID, "student-1")
	if err != nil || approved.Approved == nil || !*approved.Approved || approved.JoinedAt == nil {
		t.Fatalf("approve member: response=%#v err=%v", approved, err)
	}
	cfg.MaxStudentsPerClass = 1
	if _, err := svc.RequestJoinClassroom(ctx, "student-2", dto.RequestJoinClassroomRequest{Code: created.Code}); !errors.Is(err, service.ErrClassroomCapacityFull) {
		t.Fatalf("join full classroom: %v", err)
	}
	cfg.MaxStudentsPerClass = 50

	active := true
	studentQuery := dto.ClassroomsQuery{Active: &active}
	studentQuery.Normalize()
	studentClasses, pages, err := svc.GetMyClassrooms(ctx, "student-1", studentQuery)
	if err != nil || pages != 1 || len(studentClasses) != 1 || studentClasses[0].ID != created.ID {
		t.Fatalf("filter student classrooms: response=%#v pages=%d err=%v", studentClasses, pages, err)
	}
	teacherQuery := dto.ClassroomsQuery{Query: "advanced"}
	teacherQuery.Normalize()
	teacherClasses, _, err := svc.GetMyClassrooms(ctx, "teacher-1", teacherQuery)
	if err != nil || len(teacherClasses) != 1 || teacherClasses[0].ID != created.ID {
		t.Fatalf("filter teacher classrooms: response=%#v err=%v", teacherClasses, err)
	}
	teacherNameQuery := dto.ClassroomsQuery{Query: "teacher"}
	teacherNameQuery.Normalize()
	teacherNameClasses, _, err := svc.GetClassrooms(ctx, teacherNameQuery)
	if err != nil || len(teacherNameClasses) != 1 || teacherNameClasses[0].ID != created.ID {
		t.Fatalf("search classrooms by teacher name: response=%#v err=%v", teacherNameClasses, err)
	}

	if err := svc.RemoveMember(ctx, "teacher-1", created.ID, "student-1"); err != nil {
		t.Fatalf("remove member: %v", err)
	}
	studentQuery = dto.ClassroomsQuery{}
	studentQuery.Normalize()
	studentClasses, _, err = svc.GetMyClassrooms(ctx, "student-1", studentQuery)
	if err != nil || len(studentClasses) != 0 {
		t.Fatalf("removed member still has classroom access: response=%#v err=%v", studentClasses, err)
	}
	if _, err := svc.RequestJoinClassroom(ctx, "student-1", dto.RequestJoinClassroomRequest{Code: created.Code}); err != nil {
		t.Fatalf("request again after removal: %v", err)
	}
	archived, err := svc.ArchiveClassroom(ctx, "teacher-1", created.ID)
	if err != nil || archived.Active {
		t.Fatalf("archive classroom: response=%#v err=%v", archived, err)
	}
	if _, err := svc.RequestJoinClassroom(ctx, "student-2", dto.RequestJoinClassroomRequest{Code: created.Code}); !errors.Is(err, service.ErrClassroomNotActive) {
		t.Fatalf("join archived classroom: %v", err)
	}
	inactive := false
	archivedQuery := dto.ClassroomsQuery{Active: &inactive}
	archivedQuery.Normalize()
	archivedClasses, _, err := svc.GetMyClassrooms(ctx, "teacher-1", archivedQuery)
	if err != nil || len(archivedClasses) != 1 || archivedClasses[0].ID != created.ID {
		t.Fatalf("filter archived classrooms: response=%#v err=%v", archivedClasses, err)
	}
}
