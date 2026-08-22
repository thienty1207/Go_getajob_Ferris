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
	return newRouter(cfg, handler, promotionHandler, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

// NewAuthenticatedRouter mounts the complete runtime surface: public client
// reads plus cookie-authenticated admin auth, promotion, and job-cache routes.
func NewAuthenticatedRouter(cfg config.Config, handler *Handler, promotionHandler *PromotionHandler, authHandler *AdminAuthHandler, jobsHandler *AdminJobHandler, jobLinkHandlers ...*AdminJobLinkHandler) *gin.Engine {
	var jobLinkHandler *AdminJobLinkHandler
	if len(jobLinkHandlers) > 0 {
		jobLinkHandler = jobLinkHandlers[0]
	}
	return newRouter(cfg, handler, promotionHandler, authHandler, jobsHandler, jobLinkHandler, nil, nil, nil, nil, nil, nil)
}

func NewAuthenticatedRouterWithLocations(cfg config.Config, handler *Handler, promotionHandler *PromotionHandler, authHandler *AdminAuthHandler, jobsHandler *AdminJobHandler, jobLinkHandler *AdminJobLinkHandler, locationHandler *AdminLocationHandler, settingsHandlers ...*AdminSettingsHandler) *gin.Engine {
	var settingsHandler *AdminSettingsHandler
	if len(settingsHandlers) > 0 {
		settingsHandler = settingsHandlers[0]
	}
	return newRouter(cfg, handler, promotionHandler, authHandler, jobsHandler, jobLinkHandler, locationHandler, settingsHandler, nil, nil, nil, nil)
}

// NewAuthenticatedRouterWithClientAuth mounts the complete runtime surface plus
// the client Google login domain. Production wires the ClientAuthHandler here
// so client auth endpoints are only reachable with a real session service.
func NewAuthenticatedRouterWithClientAuth(cfg config.Config, handler *Handler, promotionHandler *PromotionHandler, authHandler *AdminAuthHandler, jobsHandler *AdminJobHandler, jobLinkHandler *AdminJobLinkHandler, locationHandler *AdminLocationHandler, settingsHandler *AdminSettingsHandler, clientAuthHandler *ClientAuthHandler, clientCVHandler *ClientCVHandler, adminClientUserHandler *AdminClientUserHandler, adminCVHandler *AdminCVHandler) *gin.Engine {
	return newRouter(cfg, handler, promotionHandler, authHandler, jobsHandler, jobLinkHandler, locationHandler, settingsHandler, clientAuthHandler, clientCVHandler, adminClientUserHandler, adminCVHandler)
}

func newRouter(cfg config.Config, handler *Handler, promotionHandler *PromotionHandler, authHandler *AdminAuthHandler, jobsHandler *AdminJobHandler, jobLinkHandler *AdminJobLinkHandler, locationHandler *AdminLocationHandler, settingsHandler *AdminSettingsHandler, clientAuthHandler *ClientAuthHandler, clientCVHandler *ClientCVHandler, adminClientUserHandler *AdminClientUserHandler, adminCVHandler *AdminCVHandler) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	if clientAuthHandler != nil {
		handler.requireClient = true
	}
	router.Use(corsMiddleware(cfg.AllowedOrigins))
	router.Use(gin.CustomRecovery(func(c *gin.Context, _ any) {
		writeError(c, http.StatusInternalServerError, "internal_error", "Service tạm thời không khả dụng.")
	}))

	readRateLimit := cfg.ReadRateLimitPerMinute
	if readRateLimit <= 0 {
		readRateLimit = 60
	}
	writeRateLimit := cfg.RateLimitPerMinute
	if writeRateLimit <= 0 {
		writeRateLimit = 30
	}
	router.GET("/healthz", handler.Health)
	if clientAuthHandler != nil {
		clientAuth := router.Group("/api/v1/client/auth")
		clientAuth.GET("/google", clientAuthHandler.Start)
		clientAuth.GET("/google/callback", clientAuthHandler.Callback)
		clientAuth.GET("/me", RequireClientSession(clientAuthHandler.auth, cfg), clientAuthHandler.Me)
		clientAuth.POST("/logout", RequireClientSession(clientAuthHandler.auth, cfg), RequireClientCSRF(clientAuthHandler.auth), clientAuthHandler.Logout)
	}
	clientScans := router.Group("/api/v1/client/scans")
	if clientAuthHandler != nil {
		clientScans.Use(RequireClientSession(clientAuthHandler.auth, cfg))
	}
	clientScans.GET("/:scan_id", rateLimitMiddleware(newRateLimiter(readRateLimit, time.Minute)), handler.GetScan)
	// A browser automatically sends the HttpOnly client cookie on a cross-site
	// form POST. Require the session's CSRF token for CV uploads so another site
	// cannot create scans or consume parsing quota on the user's behalf.
	createScanHandlers := []gin.HandlerFunc{rateLimitMiddleware(newRateLimiter(writeRateLimit, time.Minute))}
	if clientAuthHandler != nil {
		createScanHandlers = append(createScanHandlers, RequireClientCSRF(clientAuthHandler.auth))
	}
	createScanHandlers = append(createScanHandlers, handler.CreateScan)
	clientScans.POST("", createScanHandlers...)
	if clientAuthHandler != nil && clientCVHandler != nil {
		clientCVs := router.Group("/api/v1/client/cv-history")
		clientCVs.Use(RequireClientSession(clientAuthHandler.auth, cfg))
		clientCVs.GET("", rateLimitMiddleware(newRateLimiter(readRateLimit, time.Minute)), clientCVHandler.List)
		clientCVs.DELETE("/:scan_id", rateLimitMiddleware(newRateLimiter(writeRateLimit, time.Minute)), RequireClientCSRF(clientAuthHandler.auth), clientCVHandler.Delete)
	}
	if handler.homeSections != nil {
		router.GET("/api/v1/client/home-sections", rateLimitMiddleware(newRateLimiter(readRateLimit, time.Minute)), handler.homeSections.ListPublic)
	}
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
			if adminClientUserHandler != nil {
				protected.GET("/users", adminClientUserHandler.List)
			}
			if adminCVHandler != nil {
				adminCVs := protected.Group("/cv-profiles")
				adminCVs.GET("", adminCVHandler.List)
				adminCVs.DELETE("/:scan_id", RequireAdminCSRF(authHandler.auth), adminCVHandler.Delete)
			}
			if handler.homeSections != nil {
				homeSections := protected.Group("/home-sections")
				homeSections.GET("", handler.homeSections.ListAdmin)
				homeSections.PUT("/:slot", RequireAdminCSRF(authHandler.auth), handler.homeSections.Upsert)
				homeMedia := homeSections.Group("/:slot/items")
				homeMedia.POST("", RequireAdminCSRF(authHandler.auth), handler.homeSections.CreateMedia)
				homeMedia.PATCH("/:id", RequireAdminCSRF(authHandler.auth), handler.homeSections.UpdateMedia)
				homeMedia.DELETE("/:id", RequireAdminCSRF(authHandler.auth), handler.homeSections.DeleteMedia)
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
