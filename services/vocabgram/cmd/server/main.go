package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/jwt"
	"ego/platform/logger"
	"ego/platform/rpc"
	vocabConfig "ego/services/vocabgram/config"
	"ego/services/vocabgram/database"
	"ego/services/vocabgram/internal/handler"
	"ego/services/vocabgram/internal/repository"
	"ego/services/vocabgram/internal/service"

	httpSwagger "github.com/swaggo/http-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	_ "ego/services/vocabgram/docs"
)

// @title           VocabGram Service API
// @version         1.0
// @description     Unified service for Dictionary (vocabulary, phonetics, thesaurus, flashcards, topics) and Grammar (curriculum, units, practice, progress).
// @host            localhost
// @BasePath        /vocabgram/api/v1
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

	appConfig, err := vocabConfig.LoadAppConfig()
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

	identityConn, err := grpc.NewClient(
		appConfig.IdentityServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(rpc.TimeoutInterceptor(5*time.Second)),
	)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to identity service")
	}
	defer identityConn.Close()

	identityServiceClient := identityClient.NewIdentityServiceClient(identityConn)
	authMiddleware := jwt.NewAuthMiddleware(identityServiceClient)

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
	repo := repository.NewRepository(db)
	svc := service.New(repo, appConfig.AudioBaseURL)
	h := handler.New(svc)
	h.RegisterRoutes(api, authMiddleware)

	server := &http.Server{
		Addr:    ":" + appConfig.Port,
		Handler: logger.Middleware(mux),
	}

	logger.Log.Info().Str("VOCABGRAM_HTTP_PORT", appConfig.Port).Msg("[STARTUP] Starting vocabgram server")
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start vocabgram server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info().Msg("[SHUTDOWN] Shutting down vocabgram server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Log.Error().Err(err).Msg("[SHUTDOWN] Failed to shut down vocabgram server gracefully")
	}
}
