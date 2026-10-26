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

func TestClassroomsQuery(t *testing.T) {
	t.Run("accepts omitted query", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/classrooms?page=1&pageSize=12", nil)
		var query ClassroomsQuery
		if err := httpx.DecodeQuery(request, &query); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("rejects blank query", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/classrooms?query=%20%20", nil)
		var query ClassroomsQuery
		if err := httpx.DecodeQuery(request, &query); err == nil {
			t.Fatal("expected validation error")
		}
	})

	t.Run("normalizes query", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/classrooms?query=%20English%20", nil)
		var query ClassroomsQuery
		if err := httpx.DecodeQuery(request, &query); err != nil {
			t.Fatal(err)
		}
		query.Normalize()
		if query.Query != "English" {
			t.Fatalf("expected normalized query, got %q", query.Query)
		}
	})
}
