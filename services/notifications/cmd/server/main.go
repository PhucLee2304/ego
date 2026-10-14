package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	notificationsClient "ego/api/gen/go/notifications"
	tokenClient "ego/api/gen/go/token"
	"ego/platform/jwt"
	"ego/platform/logger"
	"ego/platform/rpc"
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
	cfg := config.LoadAppConfig()
	db, err := database.Connect(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[NOTIFICATIONS] Failed to connect database")
	}
	if err := database.Migrate(db); err != nil {
		logger.Log.Fatal().Err(err).Msg("[NOTIFICATIONS] Failed to migrate database")
	}

	authConn, err := grpc.NewClient(cfg.AuthServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(rpc.TimeoutInterceptor(5*time.Second)))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[NOTIFICATIONS] Failed to connect auth service")
	}
	defer authConn.Close()
	authMiddleware := jwt.NewAuthMiddleware(tokenClient.NewTokenServiceClient(authConn))

	var sender *push.FCMSender
	if cfg.FirebaseCredentialsPath != "" {
		app, firebaseErr := firebasev4.NewApp(context.Background(), nil, option.WithCredentialsFile(cfg.FirebaseCredentialsPath))
		if firebaseErr != nil {
			logger.Log.Error().Err(firebaseErr).Msg("[NOTIFICATIONS] Firebase disabled")
		} else if messagingClient, messagingErr := app.Messaging(context.Background()); messagingErr != nil {
			logger.Log.Error().Err(messagingErr).Msg("[NOTIFICATIONS] Firebase Messaging disabled")
		} else {
			sender = push.New(messagingClient)
		}
	}

	repo := repository.New(db)
	svc := service.New(repo, sender)
	h := handler.New(svc)
	api := http.NewServeMux()
	h.RegisterRoutes(api, authMiddleware)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { httpxJSON(w, http.StatusOK, `{"status":"OK"}`) })
	mux.Handle("/docs/", httpSwagger.Handler(httpSwagger.PersistAuthorization(true)))
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", api))

	server := &http.Server{Addr: ":" + cfg.Port, Handler: logger.Middleware(mux), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		logger.Log.Info().Str("port", cfg.Port).Msg("[NOTIFICATIONS] Server started")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal().Err(err).Msg("[NOTIFICATIONS] Server failed")
		}
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GRPCPort))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[NOTIFICATIONS] Failed to listen for gRPC")
	}
	grpcServer := grpc.NewServer()
	notificationsClient.RegisterNotificationServiceServer(grpcServer, notificationsRpc.New(svc))
	go func() {
		logger.Log.Info().Str("port", cfg.GRPCPort).Msg("[NOTIFICATIONS] gRPC server started")
		if err := grpcServer.Serve(lis); err != nil {
			logger.Log.Fatal().Err(err).Msg("[NOTIFICATIONS] gRPC server failed")
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	grpcServer.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func httpxJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
