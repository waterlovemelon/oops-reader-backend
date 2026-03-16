package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oops-reader/oops-reader-backend/internal/platform/config"
	"github.com/oops-reader/oops-reader-backend/internal/platform/db"
	"github.com/oops-reader/oops-reader-backend/internal/platform/log"
	"github.com/oops-reader/oops-reader-backend/internal/transport/http/handlers"
	"github.com/oops-reader/oops-reader-backend/internal/transport/http/middleware"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}

	logger := log.NewLogger(cfg)
	logger.Info("Starting Oops Reader Backend API server")

	dbPool, err := db.NewMySQL(cfg)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()

	router := setupRouter(cfg, logger, dbPool)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	go func() {
		logger.Info(fmt.Sprintf("Server listening on port %d", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

func setupRouter(cfg *config.Config, logger *zap.Logger, db *sql.DB) *gin.Engine {
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	router.Use(middleware.Logger(logger))
	router.Use(middleware.Recovery(logger))
	router.Use(middleware.CORS())

	healthHandler := handlers.NewHealthHandler(db)
	router.GET("/health", healthHandler.Check)

	api := router.Group("/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", handlers.Register)
			auth.POST("/login", handlers.Login)
			auth.POST("/refresh", handlers.RefreshToken)
		}

		users := api.Group("/users")
		users.Use(middleware.Auth(&middleware.JWTConfig{
			Secret: cfg.JWT.Secret,
		}))
		{
			users.GET("/me", handlers.GetCurrentUser)
			users.PATCH("/me", handlers.UpdateCurrentUser)
		}

		books := api.Group("/books")
		books.Use(middleware.Auth(&middleware.JWTConfig{
			Secret: cfg.JWT.Secret,
		}))
		{
			books.GET("/search", handlers.SearchBooks)
			books.GET("/:id", handlers.GetBookByID)
		}

		bookshelf := api.Group("/bookshelf")
		bookshelf.Use(middleware.Auth(&middleware.JWTConfig{
			Secret: cfg.JWT.Secret,
		}))
		{
			bookshelf.GET("", handlers.ListBookshelf)
			bookshelf.POST("", handlers.AddToBookshelf)
			bookshelf.PATCH("/:id", handlers.UpdateBookshelf)
			bookshelf.DELETE("/:id", handlers.DeleteFromBookshelf)
		}

		reading := api.Group("/reading")
		reading.Use(middleware.Auth(&middleware.JWTConfig{
			Secret: cfg.JWT.Secret,
		}))
		{
			reading.PUT("/progress", handlers.UpdateReadingProgress)
			reading.POST("/sessions", handlers.CreateReadingSession)
			reading.GET("/stats/daily", handlers.GetDailyReadingStats)
		}

		notes := api.Group("/notes")
		notes.Use(middleware.Auth(&middleware.JWTConfig{
			Secret: cfg.JWT.Secret,
		}))
		{
			notes.GET("", handlers.ListNotes)
			notes.POST("", handlers.CreateNote)
			notes.PATCH("/:id", handlers.UpdateNote)
			notes.DELETE("/:id", handlers.DeleteNote)
		}

		api.GET("/preferences", handlers.GetPreferences).Use(middleware.Auth(&middleware.JWTConfig{
			Secret: cfg.JWT.Secret,
		}))
		api.PUT("/preferences", handlers.UpdatePreferences).Use(middleware.Auth(&middleware.JWTConfig{
			Secret: cfg.JWT.Secret,
		}))

		sync := api.Group("/sync")
		sync.Use(middleware.Auth(&middleware.JWTConfig{
			Secret: cfg.JWT.Secret,
		}))
		{
			sync.POST("/push", handlers.SyncPush)
			sync.POST("/pull", handlers.SyncPull)
		}
	}

	utils := router.Group("/v1/utils")
	{
		utils.POST("/parse-book-info", handlers.ParseBookInfo)
		utils.GET("/book-cover", handlers.GetBookCover)
	}

	return router

	return router
}
