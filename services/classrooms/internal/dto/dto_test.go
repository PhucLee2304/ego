package dto

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"ego/platform/httpx"
)

func TestClassroomDescriptionAcceptsLongTextAndCanBeCleared(t *testing.T) {
	longDescription := strings.Repeat("A classroom description. ", 1000)
	for _, test := range []struct {
		name string
		body any
		want string
	}{
		{"create long description", CreateClassroomRequest{Name: "Class", Description: &longDescription}, longDescription},
		{"update long description", UpdateClassroomRequest{Description: &longDescription}, longDescription},
		{"clear description", UpdateClassroomRequest{Description: new(string)}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			body, err := json.Marshal(test.body)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest("POST", "/classrooms", strings.NewReader(string(body)))
			switch test.body.(type) {
			case CreateClassroomRequest:
				var decoded CreateClassroomRequest
				if err := httpx.DecodeJSON(request, &decoded); err != nil {
					t.Fatal(err)
				}
				if decoded.Description == nil || *decoded.Description != test.want {
					t.Fatal("description changed during decoding")
				}
			case UpdateClassroomRequest:
				var decoded UpdateClassroomRequest
				if err := httpx.DecodeJSON(request, &decoded); err != nil {
					t.Fatal(err)
				}
				if decoded.Description == nil || *decoded.Description != test.want {
					t.Fatal("description changed during decoding")
				}
			}
		})
	}
}

func TestGetClassroomByCodeQuery(t *testing.T) {
	t.Run("accepts code", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/classrooms/code?code=ABC12345", nil)
		var query GetClassroomByCodeQuery
		if err := httpx.DecodeQuery(request, &query); err != nil {
			t.Fatal(err)
		}
		if query.Code != "ABC12345" {
			t.Fatalf("expected ABC12345, got %q", query.Code)
		}
	})

	t.Run("rejects blank code", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/classrooms/code?code=%20%20", nil)
		var query GetClassroomByCodeQuery
		if err := httpx.DecodeQuery(request, &query); err == nil {
			t.Fatal("expected validation error")
		}
	})
}

func TestCreateAssignmentsRequestUsesSharedExamMetadata(t *testing.T) {
	body := `{
		"title":"Bài kiểm tra tuần 1",
		"description":"Làm trong 45 phút",
		"questions":[{
			"content":"2 + 2 = ?",
			"options":["1","2","3","4"],
			"correctOptionIndex":3
		}],
		"schedules":[
			{"classroomId":5,"duration":45,"opensAt":"2026-10-05T08:00:00Z","dueAt":"2026-10-06T08:00:00Z"},
			{"classroomId":8,"duration":60,"opensAt":"2026-10-07T08:00:00Z","dueAt":"2026-10-08T08:00:00Z"}
		]
	}`
	request := httptest.NewRequest("POST", "/classrooms/assignments", strings.NewReader(body))
	var decoded CreateAssignmentsRequest
	if err := httpx.DecodeJSON(request, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Title != "Bài kiểm tra tuần 1" {
		t.Fatalf("unexpected title: %q", decoded.Title)
	}
	if decoded.Description == nil || *decoded.Description != "Làm trong 45 phút" {
		t.Fatal("description was not decoded")
	}
	if len(decoded.Schedules) != 2 || decoded.Schedules[0].Duration != 45 || decoded.Schedules[1].ClassroomID != 8 {
		t.Fatalf("unexpected schedules: %#v", decoded.Schedules)
	}
	if len(decoded.Questions) != 1 || decoded.Questions[0].CorrectOptionIndex != 3 {
		t.Fatalf("unexpected questions: %#v", decoded.Questions)
	}
}
