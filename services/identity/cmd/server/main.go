package main

import (
	"context"
	identityClient "ego/api/gen/go/identity"
	storageClient "ego/api/gen/go/storage"
	platformConfig "ego/platform/config"
	"ego/platform/firebase"
	"ego/platform/jwt"
	"ego/platform/logger"
	platformRpc "ego/platform/rpc"
	identityConfig "ego/services/identity/config"
	"ego/services/identity/database"
	"ego/services/identity/internal/handler"
	"ego/services/identity/internal/repository"
	"ego/services/identity/internal/service"
	"ego/services/identity/internal/token"
	identityRpc "ego/services/identity/rpc"
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

	_ "ego/services/identity/docs"
)

// @title           Identity Service API
// @version         1.0
// @description     This is the API for the Identity Service.
// @host            localhost
// @BasePath        /identity/api/v1
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

	appConfig, err := identityConfig.LoadAppConfig()
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CONFIG] Failed to load App config")
	}

	jwtConfig, err := identityConfig.LoadJwtConfig()
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

	db, err := database.Connect(appConfig)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to database")
	}
	if err := database.Migrate(db); err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to migrate database")
	}

	storageConn, err := grpc.NewClient(appConfig.StorageServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(platformRpc.TimeoutInterceptor(5*time.Second)))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to storage service")
	}
	defer storageConn.Close()
	storageServiceClient := storageClient.NewStorageServiceClient(storageConn)

	jwtManager := token.NewManager(jwtConfig)
	repo := repository.NewRepository(db)
	svc := service.New(jwtManager, firebaseClient, repo, storageServiceClient)

	authMiddleware := jwt.NewAuthMiddlewareWithValidator(svc.ValidateToken)
	roleMiddleware := jwt.NewRoleMiddleware(svc.GetRole)

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
	handler.New(svc).RegisterRoutes(api, authMiddleware, roleMiddleware)

	logger.Log.Info().Str("IDENTITY_HTTP_PORT", appConfig.Port).Msg("[STARTUP] Starting identity server")
	httpServer := &http.Server{Addr: ":" + appConfig.Port, Handler: logger.Middleware(mux)}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start identity server")
		}
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", appConfig.GRPCPort))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start identity gRPC server")
	}

	rpcServer := identityRpc.New(svc)
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(platformRpc.ErrorInterceptor()))
	identityClient.RegisterIdentityServiceServer(grpcServer, rpcServer)

	logger.Log.Info().Str("IDENTITY_GRPC_PORT", appConfig.GRPCPort).Msg("[STARTUP] Starting identity gRPC server")
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to serve identity gRPC")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Log.Info().Msg("[SHUTDOWN] Shutting down identity server gracefully")

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error().Err(err).Msg("[SHUTDOWN] Failed to shut down identity HTTP server")
	}
	grpcServer.GracefulStop()
}
