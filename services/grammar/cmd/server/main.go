package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	tokenClient "ego/api/gen/go/token"
	"ego/platform/jwt"
	"ego/platform/logger"
	"ego/platform/rpc"
	grammarConfig "ego/services/grammar/config"
	"ego/services/grammar/database"
	"ego/services/grammar/internal/handler"
	"ego/services/grammar/internal/repository"
	"ego/services/grammar/internal/service"

	httpSwagger "github.com/swaggo/http-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	_ "ego/services/grammar/docs"
)

// @title           Grammar Service API
// @version         1.0
// @description     This is the API for the Grammar Service. Provides curriculum, lesson, practice, and progress tracking features.
// @host            localhost
// @BasePath        /grammar/api/v1
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

	appConfig, err := grammarConfig.LoadAppConfig()
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

	tokenConn, err := grpc.NewClient(
		appConfig.AuthServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(rpc.TimeoutInterceptor(5*time.Second)),
	)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to auth service")
	}
	defer tokenConn.Close()

	tokenServiceClient := tokenClient.NewTokenServiceClient(tokenConn)
	authMiddleware := jwt.NewAuthMiddleware(tokenServiceClient)

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"OK"}`))
	})

	// Swagger docs
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

	// API routes
	api := http.NewServeMux()
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", api))

	// Wire up service layer
	repo := repository.NewRepository(db)
	svc := service.New(repo)
	h := handler.New(svc)
	h.RegisterRoutes(api, authMiddleware)

	server := &http.Server{
		Addr:    ":" + appConfig.Port,
		Handler: logger.Middleware(mux),
	}

	logger.Log.Info().Str("GRAMMAR_HTTP_PORT", appConfig.Port).Msg("[STARTUP] Starting grammar server")
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start grammar server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info().Msg("[SHUTDOWN] Shutting down grammar server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Log.Error().Err(err).Msg("[SHUTDOWN] Failed to shut down grammar server gracefully")
	}
}
