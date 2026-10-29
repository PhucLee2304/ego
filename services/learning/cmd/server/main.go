package main

import (
	"context"
	identityClient "ego/api/gen/go/identity"
	learningClient "ego/api/gen/go/learning"
	notificationsClient "ego/api/gen/go/notifications"
	storageClient "ego/api/gen/go/storage"
	"ego/platform/jwt"
	"ego/platform/logger"
	platformRpc "ego/platform/rpc"
	platformsocket "ego/platform/socket"
	learningConfig "ego/services/learning/config"
	"ego/services/learning/database"
	"ego/services/learning/internal/handler"
	"ego/services/learning/internal/media"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/service"
	learningRpc "ego/services/learning/rpc"
	"ego/services/learning/workers"
	"errors"
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

	_ "ego/services/learning/docs"
)

// @title           Learning Service API
// @version         1.0
// @description     API của service learning (gộp exams + classrooms).
// @host            localhost
// @BasePath        /learning/api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Paste JWT token only. Swagger UI will add "Bearer" automatically.

func main() {
	logger.Setup(logger.LoggerConfig{Level: "debug", Pretty: true})

	defer func() {
		if r := recover(); r != nil {
			logger.Log.Fatal().Msg("[CRITICAL] Application panicked")
		}
	}()

	ctx := context.Background()
	appCtx, stopApp := context.WithCancel(ctx)
	defer stopApp()

	appConfig, err := learningConfig.LoadAppConfig()
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

	identityConn, err := grpc.NewClient(appConfig.IdentityServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(platformRpc.TimeoutInterceptor(5*time.Second)))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to identity service")
	}
	defer identityConn.Close()
	identityServiceClient := identityClient.NewIdentityServiceClient(identityConn)

	storageConn, err := grpc.NewClient(appConfig.StorageServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(platformRpc.TimeoutInterceptor(5*time.Second)))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to storage service")
	}
	defer storageConn.Close()
	storageServiceClient := storageClient.NewStorageServiceClient(storageConn)

	notificationsConn, err := grpc.NewClient(appConfig.NotificationsServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(platformRpc.TimeoutInterceptor(5*time.Second)))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to notifications service")
	}
	defer notificationsConn.Close()
	notificationsServiceClient := notificationsClient.NewNotificationServiceClient(notificationsConn)

	mediaClient := media.New(appConfig.LiveKitURL, appConfig.LiveKitAPIKey, appConfig.LiveKitAPISecret, appConfig.LiveKitJoinTokenTTL)

	// Hai hub WebSocket riêng: /ws/exams (attempt) và /ws/classrooms (room, gradebook).
	examsSocket := platformsocket.NewServer(identityServiceClient)
	classroomsSocket := platformsocket.NewServer(identityServiceClient)

	repo := repository.NewRepository(db)
	svc := service.New(repo, appConfig, notificationsServiceClient, identityServiceClient, storageServiceClient, mediaClient, classroomsSocket.BroadcastToUser, examsSocket.BroadcastToUser)

	authMiddleware := jwt.NewAuthMiddleware(identityServiceClient)
	roleMiddleware := jwt.NewRoleMiddleware(func(ctx context.Context, userID string) (string, error) {
		userResp, err := identityServiceClient.GetUserRole(ctx, &identityClient.GetUserRoleRequest{Id: userID})
		if err != nil {
			return "", err
		}
		return userResp.Role, nil
	})

	httpHandler := handler.New(svc)

	workers.StartNotificationOutbox(appCtx, svc, 2*time.Second, 100)
	workers.StartRoomLifecycle(appCtx, svc, 15*time.Second)
	workers.StartAutoSubmitExpiredAttempts(appCtx, svc, 5*time.Second, 100)

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
	httpHandler.RegisterRoutes(api, authMiddleware, roleMiddleware)
	httpHandler.RegisterSocketHandlers(examsSocket)
	handler.RegisterSockets(api, examsSocket, classroomsSocket)
	api.HandleFunc("POST /livekit/webhook", handler.LiveKitWebhook(svc, appConfig.LiveKitAPIKey, appConfig.LiveKitAPISecret))

	logger.Log.Info().Str("LEARNING_HTTP_PORT", appConfig.Port).Msg("[STARTUP] Starting learning server")
	httpServer := &http.Server{Addr: ":" + appConfig.Port, Handler: logger.Middleware(mux)}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start learning server")
		}
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", appConfig.GRPCPort))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start learning gRPC server")
	}

	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(platformRpc.ErrorInterceptor()))
	learningClient.RegisterLearningServiceServer(grpcServer, learningRpc.New(svc))

	logger.Log.Info().Str("LEARNING_GRPC_PORT", appConfig.GRPCPort).Msg("[STARTUP] Starting learning gRPC server")
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to serve learning gRPC")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Log.Info().Msg("[SHUTDOWN] Shutting down learning server gracefully")
	stopApp()

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error().Err(err).Msg("[SHUTDOWN] Failed to shut down learning HTTP server")
	}
	grpcServer.GracefulStop()
}
