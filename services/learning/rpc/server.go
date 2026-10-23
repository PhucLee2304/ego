package rpc

import (
	learningClient "ego/api/gen/go/learning"
	"ego/services/learning/internal/service"
)

type server struct {
	learningClient.UnimplementedLearningServiceServer
	service service.Service
}

func New(svc service.Service) learningClient.LearningServiceServer {
	return &server{service: svc}
}
