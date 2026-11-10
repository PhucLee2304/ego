package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	identityClient "ego/api/gen/go/identity"
	learningClient "ego/api/gen/go/learning"
	notificationsClient "ego/api/gen/go/notifications"
	storageClient "ego/api/gen/go/storage"
	"ego/platform/jwt"
	"ego/platform/logger"
	platformRpc "ego/platform/rpc"
	"ego/services/posts/config"
	"ego/services/posts/database"
	"ego/services/posts/internal/handler"
	"ego/services/posts/internal/repository"
	"ego/services/posts/internal/service"
	"ego/services/posts/workers"

	httpSwagger "github.com/swaggo/http-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	_ "ego/services/posts/docs"
)

// @title           Posts Service API
// @version         1.0
// @description     This is the API for the Posts Service (community and classroom posts).
// @host            localhost
// @BasePath        /posts/api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Paste JWT token only. Swagger UI will add "Bearer" automatically.

func main() {
	logger.Setup(logger.LoggerConfig{Level: "debug", Pretty: true})

	appConfig := config.LoadAppConfig()

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

	learningConn, err := grpc.NewClient(appConfig.LearningServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(platformRpc.TimeoutInterceptor(5*time.Second)))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to learning service")
	}
	defer learningConn.Close()

	notificationsConn, err := grpc.NewClient(appConfig.NotificationsServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(platformRpc.TimeoutInterceptor(5*time.Second)))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to notifications service")
	}
	defer notificationsConn.Close()

	storageConn, err := grpc.NewClient(appConfig.StorageServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(platformRpc.TimeoutInterceptor(5*time.Second)))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to storage service")
	}
	defer storageConn.Close()

	repo := repository.NewRepository(db)
	svc := service.New(
		repo,
		appConfig,
		learningClient.NewLearningServiceClient(learningConn),
		identityServiceClient,
		notificationsClient.NewNotificationServiceClient(notificationsConn),
		storageClient.NewStorageServiceClient(storageConn),
	)

	authMiddleware := jwt.NewAuthMiddleware(identityServiceClient)
	roleMiddleware := jwt.NewRoleMiddleware(func(ctx context.Context, userID string) (string, error) {
		userResp, err := identityServiceClient.GetUserRole(ctx, &identityClient.GetUserRoleRequest{Id: userID})
		if err != nil {
			return "", err
		}
		return userResp.Role, nil
	})

	api := http.NewServeMux()
	handler.New(svc).RegisterRoutes(api, authMiddleware, roleMiddleware)

	workerCtx, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	workers.StartNotificationOutbox(workerCtx, svc, 2*time.Second, 100)
	workers.StartUploadCleanup(workerCtx, svc, 5*time.Minute, 100)
	workers.StartVideoProcessor(workerCtx, svc, 2*time.Second, 5)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"OK"}`))
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
	logger.Log.Info().Msg("[SHUTDOWN] Shutting down posts server gracefully")
	stopWorkers()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error().Err(err).Msg("[SHUTDOWN] Failed to stop posts server gracefully")
	}
}
