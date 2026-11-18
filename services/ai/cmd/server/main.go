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
	"ego/platform/jwt"
	"ego/platform/logger"
	platformRpc "ego/platform/rpc"
	aiConfig "ego/services/ai/config"
	"ego/services/ai/internal/handler"
	"ego/services/ai/internal/llm"
	"ego/services/ai/internal/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger.Setup(logger.LoggerConfig{Level: "debug", Pretty: true})

	defer func() {
		if r := recover(); r != nil {
			logger.Log.Fatal().Msg("[CRITICAL] Application panicked")
		}
	}()

	appConfig := aiConfig.LoadAppConfig()

	identityConn, err := grpc.NewClient(appConfig.IdentityServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(platformRpc.TimeoutInterceptor(5*time.Second)),
	)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to identity service")
	}
	defer identityConn.Close()
	identityServiceClient := identityClient.NewIdentityServiceClient(identityConn)

	llmClient := llm.NewClient(appConfig.GeminiAPIKey, appConfig.GeminiModel)
	svc := service.New(llmClient)

	authMiddleware := jwt.NewAuthMiddleware(identityServiceClient)
	httpHandler := handler.New(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"OK"}`))
	})

	api := http.NewServeMux()
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", api))
	httpHandler.RegisterRoutes(api, authMiddleware)

	logger.Log.Info().Str("AI_HTTP_PORT", appConfig.Port).Msg("[STARTUP] Starting AI service")
	httpServer := &http.Server{Addr: ":" + appConfig.Port, Handler: logger.Middleware(mux)}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start AI server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Log.Info().Msg("[SHUTDOWN] Shutting down AI server gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error().Err(err).Msg("[SHUTDOWN] Failed to shut down AI HTTP server")
	}
}
