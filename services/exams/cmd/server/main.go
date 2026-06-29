package main

import (
	"context"
	tokenClient "ego/api/gen/go/token"
	"ego/platform/jwt"
	"ego/platform/logger"
	"ego/platform/rpc"
	examsConfig "ego/services/exams/config"
	"ego/services/exams/database"
	"ego/services/exams/internal/handler"
	"ego/services/exams/internal/repository"
	"ego/services/exams/internal/service"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpSwagger "github.com/swaggo/http-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	_ "ego/services/exams/docs"
)

// @title           Exams Service API
// @version         1.0
// @description     This is the API for the Exams Service.
// @host            localhost
// @BasePath        /exams/api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Paste JWT token only. Swagger UI will add "Bearer" automatically.

func main() {
	logger.Setup(logger.LoggerConfig{
		Level:  "debug",
		Pretty: true,
	})

	defer func() {
		if r := recover(); r != nil {
			logger.Log.Fatal().Msg("[CRITICAL] Application panicked")
		}
	}()

	_ = context.Background()
	appConfig, err := examsConfig.LoadAppConfig()
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CONFIG] Failed to load App config")
	}

	db, err := database.Connect(appConfig)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to database")
	}

	if err := database.Migrate(db); err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to migrate database")
	}

	tokenConn, err := grpc.NewClient(appConfig.AuthServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(rpc.TimeoutInterceptor(5*time.Second)))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to auth service")
	}
	defer tokenConn.Close()
	tokenServiceClient := tokenClient.NewTokenServiceClient(tokenConn)
	authMiddleware := jwt.NewAuthMiddleware(tokenServiceClient)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"OK"}`))
	})

	mux.Handle("/docs/", httpSwagger.Handler(
		httpSwagger.PersistAuthorization(true),
		httpSwagger.UIConfig(map[string]string{
			"requestInterceptor": `(req) => {
				const auth = req.headers.Authorization;
				if (auth && !auth.toLowerCase().startsWith("bearer ")) {
					req.headers.Authorization = "Bearer " + auth;
				}
				return req;
			}`,
		}),
	))

	api := http.NewServeMux()
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", api))

	repo := repository.NewRepository(db)
	svc := service.New(repo)
	hdl := handler.New(svc)

	hdl.RegisterRoutes(api, authMiddleware)

	logger.Log.Info().Str("EXAMS_HTTP_PORT", appConfig.Port).Msg("[STARTUP] Starting exams server")
	go func() {
		if err := http.ListenAndServe(":"+appConfig.Port, logger.Middleware(mux)); err != nil {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start exams server")
		}
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", appConfig.GRPCPort))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start exams gRPC server")
	}

	grpcServer := grpc.NewServer()
	// examsClient.RegisterExamsServiceServer(grpcServer, rpcServer) // To be implemented when gRPC is needed

	logger.Log.Info().Str("EXAMS_GRPC_PORT", appConfig.GRPCPort).Msg("[STARTUP] Starting exams gRPC server")
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to serve exams gRPC")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Log.Info().Msg("[SHUTDOWN] Shutting down exams server")
	grpcServer.GracefulStop()
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
}
