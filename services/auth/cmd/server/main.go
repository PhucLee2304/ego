package main

import (
	"context"
	tokenClient "ego/api/gen/go/token"
	usersClient "ego/api/gen/go/users"
	platformConfig "ego/platform/config"
	"ego/platform/firebase"
	"ego/platform/jwt"
	"ego/platform/logger"
	authConfig "ego/services/auth/config"
	authHandler "ego/services/auth/internal/handler"
	authService "ego/services/auth/internal/service"
	authRpc "ego/services/auth/rpc"
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

	_ "ego/services/auth/docs"
)

// @title           Auth Service API
// @version         1.0
// @description     This is the API for the Auth Service.
// @host            localhost
// @BasePath        /auth/api/v1
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

func main() {
	logger.Setup(logger.LoggerConfig{Level: "debug", Pretty: true})

	defer func() {
		if r := recover(); r != nil {
			logger.Log.Fatal().Msg("[CRITICAL] Application panicked")
		}
	}()

	ctx := context.Background()

	appConfig, err := authConfig.LoadAppConfig()
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CONFIG] Failed to load App config")
	}

	jwtConfig, err := platformConfig.LoadJwtConfig()
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CONFIG] Failed to load JWT config")
	}

	firebaseConfig, err := platformConfig.LoadFirebaseConfig()
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CONFIG] Failed to load Firebase config")
	}
	firebaseClient, err := firebase.NewClient(ctx, firebaseConfig)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to initialize Firebase app")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"OK"}`))
	})

	mux.Handle("/docs/", httpSwagger.WrapHandler)

	api := http.NewServeMux()
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", api))

	jwtManager := jwt.NewManager(jwtConfig)

	usersConn, err := grpc.NewClient(appConfig.UsersServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	logger.Log.Info().Str("USERS_SERVICE_ADDR", appConfig.UsersServiceAddr).Msg("[CONFIG] Connecting to users service")
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to users service")
	}
	defer usersConn.Close()
	usersClient := usersClient.NewUserServiceClient(usersConn)

	tokenConn, err := grpc.NewClient("localhost:"+appConfig.GRPCPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to token service")
	}
	defer tokenConn.Close()
	tokenServiceClient := tokenClient.NewTokenServiceClient(tokenConn)
	authMiddleware := jwt.NewAuthMiddleware(tokenServiceClient)

	service := authService.New(jwtManager, *firebaseClient, usersClient)
	handler := authHandler.New(service)
	handler.RegisterRoutes(api, authMiddleware)

	logger.Log.Info().Str("AUTH_HTTP_PORT", appConfig.Port).Msg("[STARTUP] Starting auth server")
	go func() {
		if err := http.ListenAndServe(":"+appConfig.Port, logger.Middleware(mux)); err != nil {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start auth server")
		}
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", appConfig.GRPCPort))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start auth gRPC server")
	}

	rpcServer := authRpc.New(jwtManager)
	grpcServer := grpc.NewServer()
	tokenClient.RegisterTokenServiceServer(grpcServer, rpcServer)

	logger.Log.Info().Str("AUTH_GRPC_PORT", appConfig.GRPCPort).Msg("[STARTUP] Starting auth gRPC server")
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to serve auth gRPC")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Log.Info().Msg("[SHUTDOWN] Shutting down auth server gracefully")

	grpcServer.GracefulStop()

	_, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
}
