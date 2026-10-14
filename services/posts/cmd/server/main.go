package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	classroomsClient "ego/api/gen/go/classrooms"
	notificationsClient "ego/api/gen/go/notifications"
	storageClient "ego/api/gen/go/storage"
	tokenClient "ego/api/gen/go/token"
	usersClient "ego/api/gen/go/users"
	"ego/platform/jwt"
	"ego/platform/logger"
	"ego/platform/rpc"
	"ego/services/posts/config"
	"ego/services/posts/database"
	"ego/services/posts/internal/handler"
	"ego/services/posts/internal/repository"
	"ego/services/posts/internal/service"
	"ego/services/posts/workers"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	rpcTimeout      = 5 * time.Second
	shutdownTimeout = 10 * time.Second
)

func main() {
	logger.Setup(logger.LoggerConfig{Level: "debug", Pretty: true})

	appConfig, err := config.LoadAppConfig()
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CONFIG] Failed to load app config")
	}

	db, err := database.Connect(appConfig)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[DATABASE] Failed to connect to database")
	}
	if err := database.Migrate(db); err != nil {
		logger.Log.Fatal().Err(err).Msg("[DATABASE] Failed to migrate database")
	}

	authConn, err := grpc.NewClient(
		appConfig.AuthServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(rpc.TimeoutInterceptor(rpcTimeout)),
	)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[RPC] Failed to create auth client")
	}
	defer authConn.Close()

	usersConn, err := grpc.NewClient(
		appConfig.UsersServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(rpc.TimeoutInterceptor(rpcTimeout)),
	)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[RPC] Failed to create users client")
	}
	defer usersConn.Close()

	classroomsConn, err := grpc.NewClient(
		appConfig.ClassroomsServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(rpc.TimeoutInterceptor(rpcTimeout)),
	)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[RPC] Failed to create classrooms client")
	}
	defer classroomsConn.Close()

	notificationsConn, err := grpc.NewClient(
		appConfig.NotificationsServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(rpc.TimeoutInterceptor(rpcTimeout)),
	)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[RPC] Failed to create notifications client")
	}
	defer notificationsConn.Close()

	storageConn, err := grpc.NewClient(
		appConfig.StorageServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(rpc.TimeoutInterceptor(rpcTimeout)),
	)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[RPC] Failed to create storage client")
	}
	defer storageConn.Close()

	usersServiceClient := usersClient.NewUserServiceClient(usersConn)
	authMiddleware := jwt.NewAuthMiddleware(tokenClient.NewTokenServiceClient(authConn))
	roleMiddleware := jwt.NewRoleMiddleware(func(ctx context.Context, userID string) (string, error) {
		response, err := usersServiceClient.GetUserRole(ctx, &usersClient.GetUserRoleRequest{Id: userID})
		if err != nil {
			return "", err
		}
		return response.Role, nil
	})

	repo := repository.NewRepository(db)
	storageServiceClient := storageClient.NewStorageServiceClient(storageConn)
	service := service.New(
		repo,
		appConfig,
		classroomsClient.NewClassroomServiceClient(classroomsConn),
		usersServiceClient,
		notificationsClient.NewNotificationServiceClient(notificationsConn),
		storageServiceClient,
	)

	appCtx, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	workers.StartNotificationOutbox(appCtx, service, 2*time.Second, 100)
	workers.StartUploadCleanup(appCtx, service, 5*time.Minute, 100)
	workers.StartVideoProcessor(appCtx, repo, storageServiceClient, 2*time.Second)

	api := http.NewServeMux()
	httpHandler := handler.New(service)
	httpHandler.RegisterRoutes(api, authMiddleware, roleMiddleware)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"OK"}`))
	})
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", api))

	server := &http.Server{
		Addr:              ":" + appConfig.Port,
		Handler:           logger.Middleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Log.Info().Str("POSTS_HTTP_PORT", appConfig.Port).Msg("[STARTUP] Starting posts server")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Posts server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	stopWorkers()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error().Err(err).Msg("[SHUTDOWN] Failed to stop posts server gracefully")
	}
}
