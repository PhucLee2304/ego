package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	identityClient "ego/api/gen/go/identity"
	notificationsClient "ego/api/gen/go/notifications"
	"ego/platform/jwt"
	"ego/platform/logger"
	platformRpc "ego/platform/rpc"
	platformsocket "ego/platform/socket"
	"ego/services/notifications/config"
	"ego/services/notifications/database"
	"ego/services/notifications/internal/handler"
	"ego/services/notifications/internal/push"
	"ego/services/notifications/internal/repository"
	"ego/services/notifications/internal/service"
	notificationsRpc "ego/services/notifications/rpc"

	firebasev4 "firebase.google.com/go/v4"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	_ "ego/services/notifications/docs"

	httpSwagger "github.com/swaggo/http-swagger"
)

// @title Notifications Service API
// @version 1.0
// @description Notification list and device token APIs.
// @host localhost
// @BasePath /notifications/api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

func main() {
	logger.Setup(logger.LoggerConfig{Level: "debug", Pretty: true})
	ctx := context.Background()

	cfg := config.LoadAppConfig()

	db, err := database.Connect(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to database")
	}
	if err := database.Migrate(db); err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to migrate database")
	}

	identityConn, err := grpc.NewClient(cfg.IdentityServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(platformRpc.TimeoutInterceptor(5*time.Second)))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to identity service")
	}
	defer identityConn.Close()
	identityServiceClient := identityClient.NewIdentityServiceClient(identityConn)

	var sender push.Sender
	if cfg.FirebaseCredentialsPath != "" {
		app, firebaseErr := firebasev4.NewApp(ctx, nil, option.WithCredentialsFile(cfg.FirebaseCredentialsPath))
		if firebaseErr != nil {
			logger.Log.Error().Err(firebaseErr).Msg("[STARTUP] Firebase disabled")
		} else if messagingClient, messagingErr := app.Messaging(ctx); messagingErr != nil {
			logger.Log.Error().Err(messagingErr).Msg("[STARTUP] Firebase Messaging disabled")
		} else {
			sender = push.New(messagingClient)
		}
	}

	notificationsSocket := platformsocket.NewServer(identityServiceClient)

	repo := repository.NewRepository(db)
	svc := service.New(repo, sender, notificationsSocket.BroadcastToUser)

	authMiddleware := jwt.NewAuthMiddleware(identityServiceClient)
	roleMiddleware := jwt.NewRoleMiddleware(func(ctx context.Context, userID string) (string, error) {
		userResp, err := identityServiceClient.GetUserRole(ctx, &identityClient.GetUserRoleRequest{Id: userID})
		if err != nil {
			return "", err
		}
		return userResp.Role, nil
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"OK"}`))
	})
	mux.Handle("/docs/", httpSwagger.Handler(httpSwagger.PersistAuthorization(true)))

	api := http.NewServeMux()
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", api))
	handler.New(svc).RegisterRoutes(api, authMiddleware, roleMiddleware)
	api.Handle("/ws/notifications", notificationsSocket)

	logger.Log.Info().Str("NOTIFICATIONS_HTTP_PORT", cfg.Port).Msg("[STARTUP] Starting notifications server")
	httpServer := &http.Server{Addr: ":" + cfg.Port, Handler: logger.Middleware(mux), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start notifications server")
		}
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start notifications gRPC server")
	}
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(platformRpc.ErrorInterceptor()))
	notificationsClient.RegisterNotificationServiceServer(grpcServer, notificationsRpc.New(svc, notificationsSocket.DisconnectUser))

	logger.Log.Info().Str("NOTIFICATIONS_GRPC_PORT", cfg.GRPCPort).Msg("[STARTUP] Starting notifications gRPC server")
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to serve notifications gRPC")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Log.Info().Msg("[SHUTDOWN] Shutting down notifications server gracefully")

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error().Err(err).Msg("[SHUTDOWN] Failed to shut down notifications HTTP server")
	}
	grpcServer.GracefulStop()
}
