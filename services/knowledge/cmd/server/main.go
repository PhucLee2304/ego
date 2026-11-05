package main

import (
	"context"
	identityClient "ego/api/gen/go/identity"
	"ego/platform/jwt"
	"ego/platform/logger"
	platformRpc "ego/platform/rpc"
	knowledgeConfig "ego/services/knowledge/config"
	"ego/services/knowledge/database"
	"ego/services/knowledge/internal/handler"
	"ego/services/knowledge/internal/repository"
	"ego/services/knowledge/internal/service"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpSwagger "github.com/swaggo/http-swagger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	_ "ego/services/knowledge/docs"
)

// @title           Knowledge Service API
// @version         1.0
// @description     API của service knowledge (gộp topics + dictionary + grammar).
// @host            localhost
// @BasePath        /knowledge/api/v1
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

	appConfig, err := knowledgeConfig.LoadAppConfig()
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

	identityConn, err := grpc.NewClient(appConfig.IdentityServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(platformRpc.TimeoutInterceptor(5*time.Second)))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to connect to identity service")
	}
	defer identityConn.Close()
	identityServiceClient := identityClient.NewIdentityServiceClient(identityConn)

	repo := repository.NewRepository(db)
	svc := service.New(repo)

	authMiddleware := jwt.NewAuthMiddleware(identityServiceClient)

	httpHandler := handler.New(svc)

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
	httpHandler.RegisterRoutes(api, authMiddleware)

	logger.Log.Info().Str("KNOWLEDGE_HTTP_PORT", appConfig.Port).Msg("[STARTUP] Starting knowledge server")
	httpServer := &http.Server{Addr: ":" + appConfig.Port, Handler: logger.Middleware(mux)}
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal().Err(err).Msg("[CRITICAL] Failed to start knowledge server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Log.Info().Msg("[SHUTDOWN] Shutting down knowledge server gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error().Err(err).Msg("[SHUTDOWN] Failed to shut down knowledge HTTP server")
	}
}
