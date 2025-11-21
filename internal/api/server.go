package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kafitramarna/TransisiDB/internal/backfill"
	"github.com/kafitramarna/TransisiDB/internal/config"
	"github.com/kafitramarna/TransisiDB/internal/logger"
	"github.com/kafitramarna/TransisiDB/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server represents the management API server
type Server struct {
	router         *gin.Engine
	config         *config.APIConfig
	configStore    *config.RedisStore
	backfillWorker *backfill.Worker
	httpServer     *http.Server
}

// NewServer creates a new API server
func NewServer(cfg *config.APIConfig, configStore *config.RedisStore, worker *backfill.Worker) *Server {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	server := &Server{
		router:         router,
		config:         cfg,
		configStore:    configStore,
		backfillWorker: worker,
	}

	server.setupRoutes()

	return server
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Prometheus metrics endpoint (public - no auth for scraping)
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Health check (public)
	s.router.GET("/health", s.handleHealth)

	// API v1 routes (protected)
	v1 := s.router.Group("/api/v1")
	v1.Use(s.authMiddleware())
	v1.Use(s.metricsMiddleware()) // Track API metrics
	v1.Use(s.loggingMiddleware()) // Log API requests
	{
		// Configuration endpoints
		v1.GET("/config", s.handleGetConfig)
		v1.PUT("/config", s.handleUpdateConfig)
		v1.POST("/config/reload", s.handleReloadConfig)

		// Backfill endpoints
		v1.POST("/backfill/start", s.handleBackfillStart)
		v1.POST("/backfill/pause", s.handleBackfillPause)
		v1.POST("/backfill/resume", s.handleBackfillResume)
		v1.POST("/backfill/stop", s.handleBackfillStop)
		v1.GET("/backfill/status", s.handleBackfillStatus)

		// Table configuration endpoints
		v1.GET("/tables", s.handleListTables)
		v1.GET("/tables/:name", s.handleGetTable)
		v1.PUT("/tables/:name", s.handleUpdateTable)
		v1.DELETE("/tables/:name", s.handleDeleteTable)
	}

	// API v2 routes (v2.0 features)
	v2 := s.router.Group("/api/v2")
	v2.Use(s.authMiddleware())
	v2.Use(s.metricsMiddleware())
	v2.Use(s.loggingMiddleware())
	{
		// All v1 endpoints are also available in v2
		v2.GET("/config", s.handleGetConfig)
		v2.PUT("/config", s.handleUpdateConfig)
		v2.POST("/config/reload", s.handleReloadConfig)

		// v2.0: TLS/SSL status and configuration
		v2.GET("/tls/status", s.handleTLSStatus)
		v2.GET("/tls/certificates", s.handleTLSCertificates)

		// v2.0: Read replica status and health
		v2.GET("/replica/status", s.handleReplicaStatus)
		v2.GET("/replica/health", s.handleReplicaHealth)

		// v2.0: Currency detection configuration
		v2.GET("/detection/config", s.handleDetectionConfig)
		v2.PUT("/detection/config", s.handleUpdateDetectionConfig)

		// v2.0: Enhanced metrics
		v2.GET("/metrics/summary", s.handleMetricsSummary)

		// Backfill endpoints (same as v1)
		v2.POST("/backfill/start", s.handleBackfillStart)
		v2.POST("/backfill/pause", s.handleBackfillPause)
		v2.POST("/backfill/resume", s.handleBackfillResume)
		v2.POST("/backfill/stop", s.handleBackfillStop)
		v2.GET("/backfill/status", s.handleBackfillStatus)

		// Table configuration (same as v1)
		v2.GET("/tables", s.handleListTables)
		v2.GET("/tables/:name", s.handleGetTable)
		v2.PUT("/tables/:name", s.handleUpdateTable)
		v2.DELETE("/tables/:name", s.handleDeleteTable)
	}
}

// authMiddleware validates API key
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("Authorization")

		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Missing Authorization header",
			})
			c.Abort()
			return
		}

		// Remove "Bearer " prefix if present
		if len(apiKey) > 7 && apiKey[:7] == "Bearer " {
			apiKey = apiKey[7:]
		}

		if apiKey != s.config.APIKey {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API key",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// metricsMiddleware tracks API request metrics
func (s *Server) metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		status := fmt.Sprintf("%d", c.Writer.Status())
		metrics.RecordAPIRequest(c.FullPath(), c.Request.Method, status)
		metrics.RecordQueryDuration("api_request", duration)
	}
}

// loggingMiddleware logs API requests
func (s *Server) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Log request details
		duration := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		if raw != "" {
			path = path + "?" + raw
		}

		msg := "API Request"
		fields := []any{
			"status", status,
			"method", method,
			"path", path,
			"ip", clientIP,
			"latency", duration,
			"user_agent", c.Request.UserAgent(),
		}

		if status >= 500 {
			logger.Error(msg, fields...)
		} else if status >= 400 {
			logger.Warn(msg, fields...)
		} else {
			logger.Info(msg, fields...)
		}
	}
}

// Health check handler
func (s *Server) handleHealth(c *gin.Context) {
	health := gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	}

	// Check Redis health if available
	if s.configStore != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := s.configStore.Health(ctx); err != nil {
			health["redis"] = "unhealthy"
			health["status"] = "degraded"
		} else {
			health["redis"] = "healthy"
		}
	}

	c.JSON(http.StatusOK, health)
}

// Get configuration
func (s *Server) handleGetConfig(c *gin.Context) {
	ctx := context.Background()

	cfg, err := s.configStore.LoadConfig(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to load config: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, cfg)
}

// Update configuration
func (s *Server) handleUpdateConfig(c *gin.Context) {
	var newConfig config.Config

	if err := c.ShouldBindJSON(&newConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid config format: %v", err),
		})
		return
	}

	// Validate configuration
	if err := newConfig.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Configuration validation failed: %v", err),
		})
		return
	}

	ctx := context.Background()

	// Save to Redis
	if err := s.configStore.SaveConfig(ctx, &newConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to save config: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Configuration updated successfully",
		"timestamp": time.Now().Unix(),
	})
}

// Reload configuration
func (s *Server) handleReloadConfig(c *gin.Context) {
	ctx := context.Background()

	if err := s.configStore.PublishReload(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to publish reload: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Reload notification published",
		"timestamp": time.Now().Unix(),
	})
}

// Start backfill (placeholder - requires table name in request)
func (s *Server) handleBackfillStart(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "Backfill start requires integration with worker manager",
		"message": "Use standalone CLI tool for now: transisidb-backfill",
	})
}

// Pause backfill
func (s *Server) handleBackfillPause(c *gin.Context) {
	if s.backfillWorker == nil || !s.backfillWorker.IsRunning() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No backfill job is currently running",
		})
		return
	}

	if err := s.backfillWorker.Pause(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to pause backfill: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Backfill paused successfully",
	})
}

// Resume backfill
func (s *Server) handleBackfillResume(c *gin.Context) {
	if s.backfillWorker == nil || !s.backfillWorker.IsRunning() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No backfill job is currently running",
		})
		return
	}

	if err := s.backfillWorker.Resume(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to resume backfill: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Backfill resumed successfully",
	})
}

// Stop backfill
func (s *Server) handleBackfillStop(c *gin.Context) {
	if s.backfillWorker == nil || !s.backfillWorker.IsRunning() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "No backfill job is currently running",
		})
		return
	}

	s.backfillWorker.Stop()

	c.JSON(http.StatusOK, gin.H{
		"message": "Backfill stop signal sent",
	})
}

// Get backfill status
func (s *Server) handleBackfillStatus(c *gin.Context) {
	if s.backfillWorker == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "no_worker",
			"message": "Backfill worker not initialized",
		})
		return
	}

	snapshot := s.backfillWorker.GetProgress().GetSnapshot()

	c.JSON(http.StatusOK, snapshot)
}

// List all tables
func (s *Server) handleListTables(c *gin.Context) {
	ctx := context.Background()

	tables, err := s.configStore.ListTables(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to list tables: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tables": tables,
		"count":  len(tables),
	})
}

// Get table configuration
func (s *Server) handleGetTable(c *gin.Context) {
	tableName := c.Param("name")
	ctx := context.Background()

	tableConfig, err := s.configStore.LoadTableConfig(ctx, tableName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("Table not found: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, tableConfig)
}

// Update table configuration
func (s *Server) handleUpdateTable(c *gin.Context) {
	tableName := c.Param("name")

	var tableConfig config.TableConfig
	if err := c.ShouldBindJSON(&tableConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid table config: %v", err),
		})
		return
	}

	ctx := context.Background()

	if err := s.configStore.SaveTableConfig(ctx, tableName, tableConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to save table config: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Table '%s' configuration updated", tableName),
	})
}

// Delete table configuration
func (s *Server) handleDeleteTable(c *gin.Context) {
	tableName := c.Param("name")
	ctx := context.Background()

	if err := s.configStore.DeleteTableConfig(ctx, tableName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to delete table config: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Table '%s' configuration deleted", tableName),
	})
}

// Start starts the API server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("API server listening", "address", addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}
