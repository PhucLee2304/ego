package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tokenClient "ego/api/gen/go/token"
	"ego/platform/jwt"
	"ego/platform/logger"
	"ego/platform/rpc"
	dictConfig "ego/services/dictionary/config"
	"ego/services/dictionary/database"
	"ego/services/dictionary/internal/handler"
	"ego/services/dictionary/internal/repository"
	"ego/services/dictionary/internal/service"

	httpSwagger "github.com/swaggo/http-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	_ "ego/services/dictionary/docs"
)

// @title           Dictionary Service API
// @version         1.0
// @description     This is the API for the Dictionary Service. Provides word lookup, phonetics, thesaurus, and flashcard features.
// @host            localhost
// @BasePath        /dictionary/api/v1
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

	appConfig, err := dictConfig.LoadAppConfig()
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

	// Static audio files — served at /assets/
	audioDir := appConfig.AudioBasePath
	absAudioDir, _ := filepath.Abs(audioDir)
	logger.Log.Info().Str("audio_dir", absAudioDir).Msg("[STARTUP] Serving audio files")
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(absAudioDir))))

	// API routes
	api := http.NewServeMux()
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", api))

	// Wire up service layer
	// audioBaseURL is used to build full audio URLs returned in responses
	audioBaseURL := "/dictionary/assets"
	repo := repository.NewRepository(db)
	svc := service.New(repo, audioBaseURL)
	h := handler.New(svc)
	h.RegisterRoutes(api, authMiddleware)

	server := &http.Server{
		Addr:    ":" + appConfig.Port,
		Handler: logger.Middleware(mux),
	}

	logger.Log.Info().Str("DICTIONARY_HTTP_PORT", appConfig.Port).Msg("[STARTUP] Starting dictionary server")
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start dictionary server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info().Msg("[SHUTDOWN] Shutting down dictionary server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Log.Error().Err(err).Msg("[SHUTDOWN] Failed to shut down dictionary server gracefully")
	}
}
