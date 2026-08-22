package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/cloudinary"
	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/database"
	"github.com/gogetsomefoodferris/backend/internal/httpapi"
	"github.com/gogetsomefoodferris/backend/internal/processor"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/gogetsomefoodferris/backend/internal/service"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	// LoadLocal supports the developer's backend/.env while still allowing
	// DATABASE_URL and other process-level values to override it in deploys.
	cfg, err := config.LoadLocal()
	if err != nil {
		logger.Error("invalid API configuration", "err", err)
		os.Exit(1)
	}

	// Fail fast during startup so the API never advertises a healthy process
	// while its PostgreSQL dependency is unavailable.
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)
	pool, err := database.Open(startupCtx, cfg.DatabaseURL)
	cancelStartup()
	if err != nil {
		logger.Error("database connection failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Repositories own SQL and persistence shape; services own validation and
	// lifecycle rules; handlers own HTTP status and response contracts.
	scanRepository := repository.NewPostgresScanRepository(pool)
	var scanProcessor processor.ScanProcessor = processor.UnavailableProcessor{}
	if cfg.DeepSeekAPIKey != "" {
		profileParser, parserErr := processor.NewDeepSeekParser(processor.DeepSeekConfig{
			APIKey:        cfg.DeepSeekAPIKey,
			BaseURL:       cfg.DeepSeekBaseURL,
			PrimaryModel:  cfg.DeepSeekPrimaryModel,
			FallbackModel: cfg.DeepSeekFallbackModel,
		})
		if parserErr != nil {
			logger.Error("DeepSeek configuration failed", "err", parserErr)
			os.Exit(1)
		}
		scanProcessor = processor.NewMatchingProcessor(scanRepository, profileParser)
	} else {
		logger.Warn("DEEPSEEK_API_KEY is not configured; CV scans will fail without fabricating profiles")
	}
	scanService := service.NewScanService(scanRepository, scanProcessor, cfg)
	defer scanService.Close()
	promotionRepository := repository.NewPostgresPromotionRepository(pool)
	if cfg.CloudinaryURL == "" {
		logger.Error("CLOUDINARY_URL is required for the promotion feature")
		os.Exit(1)
	}
	promotionAssets, err := cloudinary.NewStore(cfg.CloudinaryURL)
	if err != nil {
		logger.Error("Cloudinary configuration failed", "err", err)
		os.Exit(1)
	}
	promotionService := service.NewPromotionService(promotionRepository, cfg, promotionAssets)
	handler := httpapi.NewHandler(scanService, pool, cfg)
	promotionHandler := httpapi.NewPromotionHandler(promotionService, cfg)
	authRepository := repository.NewPostgresAdminRepository(pool)
	authService := service.NewAdminAuthService(authRepository, cfg)
	authHandler := httpapi.NewAdminAuthHandler(authService, cfg)
	jobRepository := repository.NewPostgresJobRepository(pool)
	jobHandler := httpapi.NewAdminJobHandler(jobRepository)
	jobLinkRepository := repository.NewPostgresJobLinkRepository(pool)
	jobLinkService := service.NewJobLinkService(jobLinkRepository)
	crawlRequestRepository := repository.NewPostgresCrawlRequestRepository(pool)
	crawlRequestService := service.NewCrawlRequestService(crawlRequestRepository)
	jobLinkHandler := httpapi.NewAdminJobLinkHandler(jobLinkService, crawlRequestService)
	locationRepository := repository.NewPostgresLocationRepository(pool)
	locationService := service.NewLocationService(locationRepository)
	locationHandler := httpapi.NewAdminLocationHandler(locationService)
	settingsRepository := repository.NewPostgresSettingsRepository(pool)
	settingsService := service.NewSettingsService(settingsRepository)
	settingsHandler := httpapi.NewAdminSettingsHandler(settingsService)
	clientAuthRepository := repository.NewPostgresClientAuthRepository(pool)
	clientAuthService := service.NewClientAuthService(clientAuthRepository, cfg)
	clientAuthHandler := httpapi.NewClientAuthHandler(clientAuthService, cfg)
	clientCVRepository := repository.NewPostgresClientCVRepository(pool)
	clientCVService := service.NewClientCVService(clientCVRepository)
	clientCVHandler := httpapi.NewClientCVHandler(clientCVService)
	homeSectionRepository := repository.NewPostgresHomeSectionRepository(pool)
	homeSectionService := service.NewHomeSectionService(homeSectionRepository, cfg, promotionAssets)
	// Home image deletion is decoupled from HTTP requests through a durable
	// PostgreSQL queue. Starting the consumer here also resumes cleanup jobs
	// left by a prior crash or provider outage.
	homeSectionService.StartCleanupWorker()
	defer homeSectionService.Close()
	homeSectionHandler := httpapi.NewHomeSectionHandler(homeSectionService, cfg.MaxPromotionImageBytes)
	handler.SetHomeSectionHandler(homeSectionHandler)
	adminClientUserRepository := repository.NewPostgresAdminClientUserRepository(pool)
	adminClientUserHandler := httpapi.NewAdminClientUserHandler(adminClientUserRepository)
	adminCVRepository := repository.NewPostgresAdminCVRepository(pool)
	adminCVHandler := httpapi.NewAdminCVHandler(adminCVRepository)
	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           httpapi.NewAuthenticatedRouterWithClientAuth(cfg, handler, promotionHandler, authHandler, jobHandler, jobLinkHandler, locationHandler, settingsHandler, clientAuthHandler, clientCVHandler, adminClientUserHandler, adminCVHandler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Keep server startup asynchronous so the main goroutine can also listen for
	// OS shutdown signals and perform a bounded graceful shutdown.
	serverErrors := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-serverErrors:
		logger.Error("API server stopped unexpectedly", "err", err)
	case <-stop:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("API graceful shutdown failed", "err", err)
		}
	}
}
