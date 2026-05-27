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
	"github.com/oops-reader/oops-reader-backend/internal/backup"
	"github.com/oops-reader/oops-reader-backend/internal/catalog"
	"github.com/oops-reader/oops-reader-backend/internal/community"
	"github.com/oops-reader/oops-reader-backend/internal/entitlement"
	"github.com/oops-reader/oops-reader-backend/internal/identity"
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

	var identityStore identity.Store
	if db == nil {
		identityStore = identity.NewMemoryStore()
	} else {
		identityStore = identity.NewMySQLStore(db)
	}
	identityService := identity.NewService(identityStore, cfg.JWT.Secret)
	catalogService := catalog.NewServiceWithDB(catalog.DefaultRoot(), db)
	communityService := community.NewService()
	entitlementService := entitlement.NewService(nil)
	backupService := backup.NewService(backup.NewMySQLStore(db))

	identityHandler := handlers.NewIdentityHandler(identityService)
	catalogHandler := handlers.NewCatalogHandler(catalogService)
	communityHandler := handlers.NewCommunityHandler(communityService)
	entitlementHandler := handlers.NewEntitlementHandler(identityService, entitlementService)
	backupHandler := handlers.NewBackupHandler(backupService)
	authRequired := middleware.Auth(identityService)

	api := router.Group("/v1")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", identityHandler.Register)
			auth.POST("/login", identityHandler.Login)
			auth.POST("/refresh", identityHandler.Refresh)
			auth.POST("/logout", authRequired, identityHandler.Logout)
			auth.POST("/password/reset-request", identityHandler.RequestPasswordReset)
			auth.POST("/password/reset-confirm", identityHandler.ConfirmPasswordReset)
		}

		users := api.Group("/users")
		users.Use(authRequired)
		{
			users.GET("/me", identityHandler.GetCurrentUser)
			users.PATCH("/me", identityHandler.UpdateCurrentUser)
		}

		account := api.Group("/account")
		account.Use(authRequired)
		{
			account.GET("/entitlements", entitlementHandler.List)
		}

		backupRoutes := api.Group("/backup")
		backupRoutes.Use(authRequired)
		{
			backupRoutes.GET("/reading-data/summary", backupHandler.Summary)
			backupRoutes.POST("/reading-data", backupHandler.Upload)
			backupRoutes.GET("/reading-data", backupHandler.Download)
		}

		catalogRoutes := api.Group("/catalog")
		{
			catalogRoutes.GET("/books", catalogHandler.ListBooks)
			catalogRoutes.GET("/books/:id", catalogHandler.GetBook)
			catalogRoutes.GET("/books/:id/cover", catalogHandler.Cover)
			catalogRoutes.GET("/books/:id/download", catalogHandler.Download)
			catalogRoutes.HEAD("/books/:id/download", catalogHandler.Download)
			catalogRoutes.GET("/books/:id/manifest", catalogHandler.Manifest)
			catalogRoutes.GET("/books/:id/chapters/:chapter_id", catalogHandler.Chapter)
		}

		communityRoutes := api.Group("/community")
		{
			communityRoutes.GET("/boards", communityHandler.ListBoards)
			communityRoutes.GET("/threads", communityHandler.ListThreads)
			communityRoutes.POST("/threads", authRequired, communityHandler.CreateThread)
			communityRoutes.GET("/threads/:id", communityHandler.GetThread)
			communityRoutes.POST("/threads/:id/comments", authRequired, communityHandler.AddComment)
			communityRoutes.POST("/reactions", authRequired, communityHandler.React)
		}

		books := api.Group("/books")
		books.Use(authRequired)
		{
			books.GET("/search", handlers.SearchBooks)
			books.GET("/:id", handlers.GetBookByID)
		}

		bookshelf := api.Group("/bookshelf")
		bookshelf.Use(authRequired)
		{
			bookshelf.GET("", handlers.ListBookshelf)
			bookshelf.POST("", handlers.AddToBookshelf)
			bookshelf.PATCH("/:id", handlers.UpdateBookshelf)
			bookshelf.DELETE("/:id", handlers.DeleteFromBookshelf)
		}

		reading := api.Group("/reading")
		reading.Use(authRequired)
		{
			reading.PUT("/progress", handlers.UpdateReadingProgress)
			reading.POST("/sessions", handlers.CreateReadingSession)
			reading.GET("/stats/daily", handlers.GetDailyReadingStats)
		}

		notes := api.Group("/notes")
		notes.Use(authRequired)
		{
			notes.GET("", handlers.ListNotes)
			notes.POST("", handlers.CreateNote)
			notes.PATCH("/:id", handlers.UpdateNote)
			notes.DELETE("/:id", handlers.DeleteNote)
		}

		api.GET("/preferences", authRequired, handlers.GetPreferences)
		api.PUT("/preferences", authRequired, handlers.UpdatePreferences)

		sync := api.Group("/sync")
		sync.Use(authRequired)
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
}
