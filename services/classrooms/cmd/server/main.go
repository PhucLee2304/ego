package main

import (
	"context"
	classroomsClient "ego/api/gen/go/classrooms"
	examsClient "ego/api/gen/go/exams"
	tokenClient "ego/api/gen/go/token"
	usersClient "ego/api/gen/go/users"
	"ego/platform/jwt"
	"ego/platform/logger"
	"ego/platform/rpc"
	platformsocket "ego/platform/socket"
	classroomsConfig "ego/services/classrooms/config"
	"ego/services/classrooms/database"
	"ego/services/classrooms/internal/handler"
	"ego/services/classrooms/internal/repository"
	"ego/services/classrooms/internal/service"
	classroomsRpc "ego/services/classrooms/rpc"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	httpSwagger "github.com/swaggo/http-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	_ "ego/services/classrooms/docs"
)

// @title           Classrooms Service API
// @version         1.0
// @description     This is the API for the Classrooms Service.
// @host            localhost
// @BasePath        /classrooms/api/v1
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
			logger.Log.Fatal().Str("panic", fmt.Sprint(r)).Bytes("stack", debug.Stack()).Msg("[CRITICAL] Application panicked")
		}
	}()

	appConfig, err := classroomsConfig.LoadAppConfig()
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

	usersConn, err := grpc.NewClient(appConfig.UsersServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(rpc.TimeoutInterceptor(5*time.Second)))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to users service")
	}
	defer usersConn.Close()
	usersServiceClient := usersClient.NewUserServiceClient(usersConn)
	roleMiddleware := jwt.NewRoleMiddleware(func(ctx context.Context, userID string) (string, error) {
		resp, err := usersServiceClient.GetUserRole(ctx, &usersClient.GetUserRoleRequest{Id: userID})
		if err != nil {
			return "", err
		}
		return resp.Role, nil
	})

	examsConn, err := grpc.NewClient(appConfig.ExamsServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(rpc.TimeoutInterceptor(5*time.Second)))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to exams service")
	}
	defer examsConn.Close()
	examsServiceClient := examsClient.NewExamServiceClient(examsConn)

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
	service := service.New(repo, appConfig, usersServiceClient, examsServiceClient)
	handler := handler.New(service)
	socketServer := platformsocket.NewServer(tokenServiceClient)

	handler.RegisterRoutes(api, authMiddleware, roleMiddleware)
	api.Handle("/ws/classrooms", socketServer)

	server := &http.Server{
		Addr:    ":" + appConfig.Port,
		Handler: logger.Middleware(mux),
	}

	logger.Log.Info().Str("CLASSROOMS_HTTP_PORT", appConfig.Port).Msg("[STARTUP] Starting classrooms server")
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start classrooms server")
		}
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", appConfig.GRPCPort))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start classrooms gRPC server")
	}

	rpcServer := classroomsRpc.New(repo, socketServer.BroadcastToUser)
	grpcServer := grpc.NewServer()
	classroomsClient.RegisterClassroomServiceServer(grpcServer, rpcServer)

	logger.Log.Info().Str("CLASSROOMS_GRPC_PORT", appConfig.GRPCPort).Msg("[STARTUP] Starting classrooms gRPC server")
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to serve classrooms gRPC")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info().Msg("[SHUTDOWN] Shutting down classrooms server")
	grpcServer.GracefulStop()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Log.Error().Err(err).Msg("[SHUTDOWN] Failed to shut down classrooms server")
	}
}
