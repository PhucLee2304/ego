package rpc

import (
	learningClient "ego/api/gen/go/learning"
	"ego/services/learning/internal/service"
)

type server struct {
	learningClient.UnimplementedLearningServiceServer
	service        service.Service
	disconnectUser func(userID string) int
}

func New(svc service.Service, disconnectUser func(userID string) int) learningClient.LearningServiceServer {
	return &server{service: svc, disconnectUser: disconnectUser}
}
