package main

import (
	"context"
	tokenClient "ego/api/gen/go/token"
	"ego/platform/jwt"
	"ego/platform/logger"
	storageConfig "ego/services/storage/config"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpSwagger "github.com/swaggo/http-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// @title           Storage Service API
// @version         1.0
// @description     This is the API for the Storage Service.
// @host            localhost
// @BasePath        /storage/api/v1
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

	appConfig, err := storageConfig.LoadAppConfig()
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CONFIG] Failed to load App config")
	}

	tokenConn, err := grpc.NewClient(appConfig.AuthServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to auth service")
	}
	defer tokenConn.Close()

	tokenServiceClient := tokenClient.NewTokenServiceClient(tokenConn)
	authMiddleware := jwt.NewAuthMiddleware(tokenServiceClient)
	_ = authMiddleware

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

	server := &http.Server{
		Addr:    ":" + appConfig.Port,
		Handler: logger.Middleware(mux),
	}

	logger.Log.Info().Str("STORAGE_HTTP_PORT", appConfig.Port).Msg("[STARTUP] Starting storage server")
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start storage server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info().Msg("[SHUTDOWN] Shutting down storage server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Log.Error().Err(err).Msg("[SHUTDOWN] Failed to shut down storage server")
	}
}
