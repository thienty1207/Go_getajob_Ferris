package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gogetsomefoodferris/backend/internal/config"
)

// NewRouter keeps the existing client-only constructor available to focused
// tests and callers that do not need admin dependencies. Production uses
// NewAuthenticatedRouter below so admin routes cannot accidentally be mounted
// without a real session service.
func NewRouter(cfg config.Config, handler *Handler, promotionHandlers ...*PromotionHandler) *gin.Engine {
	var promotionHandler *PromotionHandler
	if len(promotionHandlers) > 0 {
		promotionHandler = promotionHandlers[0]
	}
	return newRouter(cfg, handler, promotionHandler, nil, nil, nil, nil, nil)
}

// NewAuthenticatedRouter mounts the complete runtime surface: public client
// reads plus cookie-authenticated admin auth, promotion, and job-cache routes.
func NewAuthenticatedRouter(cfg config.Config, handler *Handler, promotionHandler *PromotionHandler, authHandler *AdminAuthHandler, jobsHandler *AdminJobHandler, jobLinkHandlers ...*AdminJobLinkHandler) *gin.Engine {
	var jobLinkHandler *AdminJobLinkHandler
	if len(jobLinkHandlers) > 0 {
		jobLinkHandler = jobLinkHandlers[0]
	}
	return newRouter(cfg, handler, promotionHandler, authHandler, jobsHandler, jobLinkHandler, nil, nil)
}

func NewAuthenticatedRouterWithLocations(cfg config.Config, handler *Handler, promotionHandler *PromotionHandler, authHandler *AdminAuthHandler, jobsHandler *AdminJobHandler, jobLinkHandler *AdminJobLinkHandler, locationHandler *AdminLocationHandler, settingsHandlers ...*AdminSettingsHandler) *gin.Engine {
	var settingsHandler *AdminSettingsHandler
	if len(settingsHandlers) > 0 {
		settingsHandler = settingsHandlers[0]
	}
	return newRouter(cfg, handler, promotionHandler, authHandler, jobsHandler, jobLinkHandler, locationHandler, settingsHandler)
}

func newRouter(cfg config.Config, handler *Handler, promotionHandler *PromotionHandler, authHandler *AdminAuthHandler, jobsHandler *AdminJobHandler, jobLinkHandler *AdminJobLinkHandler, locationHandler *AdminLocationHandler, settingsHandler *AdminSettingsHandler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(corsMiddleware(cfg.AllowedOrigins))
	router.Use(gin.CustomRecovery(func(c *gin.Context, _ any) {
		writeError(c, http.StatusInternalServerError, "internal_error", "Service tạm thời không khả dụng.")
	}))

	readRateLimit := cfg.ReadRateLimitPerMinute
	if readRateLimit <= 0 {
		readRateLimit = 60
	}
	router.GET("/healthz", handler.Health)
	router.GET(
		"/api/v1/client/scans/:scan_id",
		rateLimitMiddleware(newRateLimiter(readRateLimit, time.Minute)),
		handler.GetScan,
	)
	router.POST(
		"/api/v1/client/scans",
		rateLimitMiddleware(newRateLimiter(cfg.RateLimitPerMinute, time.Minute)),
		handler.CreateScan,
	)
	if locationHandler != nil {
		router.GET(
			"/api/v1/client/locations",
			rateLimitMiddleware(newRateLimiter(readRateLimit, time.Minute)),
			locationHandler.ListPublic,
		)
	}
	if promotionHandler != nil {
		promotionReadRateLimit := readRateLimit
		promotionWriteRateLimit := cfg.PromotionRateLimitPerMinute
		if promotionWriteRateLimit <= 0 {
			promotionWriteRateLimit = 10
		}

		// Public reads have their own limiter because they are expected to be
		// called by every client page load and never need admin credentials.
		clientPromotions := router.Group("/api/v1/client/promotions")
		clientPromotions.Use(rateLimitMiddleware(newRateLimiter(promotionReadRateLimit, time.Minute)))
		clientPromotions.GET("", promotionHandler.List)
		clientPromotions.GET("/:slot/image", promotionHandler.Image)

		if authHandler != nil {
			adminGroup := router.Group("/api/v1/admin")
			loginRateLimit := cfg.AdminLoginRateLimitPerMinute
			if loginRateLimit <= 0 {
				loginRateLimit = 5
			}
			authGroup := adminGroup.Group("/auth")
			authGroup.POST("/login", rateLimitMiddleware(newRateLimiter(loginRateLimit, time.Minute)), authHandler.Login)

			protected := adminGroup.Group("")
			protected.Use(RequireAdminSession(authHandler.auth, cfg))
			protected.GET("/auth/me", authHandler.Me)
			protected.POST("/auth/logout", RequireAdminCSRF(authHandler.auth), authHandler.Logout)
			if settingsHandler != nil {
				protected.GET("/settings", settingsHandler.Get)
				protected.PATCH("/settings/crawler", RequireAdminCSRF(authHandler.auth), settingsHandler.UpdateCrawler)
			}

			adminPromotions := protected.Group("/promotions")
			adminPromotions.Use(rateLimitMiddleware(newRateLimiter(promotionWriteRateLimit, time.Minute)))
			adminPromotions.GET("", promotionHandler.ListAdmin)
			adminPromotions.PUT("/:slot", RequireAdminCSRF(authHandler.auth), promotionHandler.Upsert)
			adminPromotions.DELETE("/:slot", RequireAdminCSRF(authHandler.auth), promotionHandler.Delete)

			if jobsHandler != nil {
				adminJobs := protected.Group("/jobs")
				adminJobs.Use(rateLimitMiddleware(newRateLimiter(promotionReadRateLimit, time.Minute)))
				adminJobs.GET("", jobsHandler.List)
			}
			if jobLinkHandler != nil {
				adminJobLinks := protected.Group("/job-links")
				adminJobLinks.Use(rateLimitMiddleware(newRateLimiter(promotionReadRateLimit, time.Minute)))
				adminJobLinks.GET("", jobLinkHandler.List)
				adminJobLinks.POST("", RequireAdminCSRF(authHandler.auth), jobLinkHandler.Create)
				adminJobLinks.PATCH("/:id", RequireAdminCSRF(authHandler.auth), jobLinkHandler.Update)
				adminJobLinks.PATCH("/:id/status", RequireAdminCSRF(authHandler.auth), jobLinkHandler.SetStatus)
				if jobLinkHandler.crawl != nil {
					adminJobLinks.POST("/:id/crawl", RequireAdminCSRF(authHandler.auth), jobLinkHandler.Crawl)
				}
				adminJobLinks.DELETE("/:id", RequireAdminCSRF(authHandler.auth), jobLinkHandler.Delete)
			}
			if locationHandler != nil {
				adminLocations := protected.Group("/locations")
				adminLocations.GET("", locationHandler.List)
				adminLocations.GET("/options", locationHandler.Options)
				adminLocations.POST("", RequireAdminCSRF(authHandler.auth), locationHandler.Create)
				adminLocations.PATCH("/:id", RequireAdminCSRF(authHandler.auth), locationHandler.Update)
				adminJobs := protected.Group("/jobs")
				adminJobs.PATCH("/:id/location", RequireAdminCSRF(authHandler.auth), locationHandler.AssignJobLocation)
			}
		}
	}
	return router
}
