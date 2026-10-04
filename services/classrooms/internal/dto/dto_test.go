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
