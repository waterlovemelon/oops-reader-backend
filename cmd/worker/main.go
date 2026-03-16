package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oops-reader/oops-reader-backend/internal/platform/config"
	"github.com/oops-reader/oops-reader-backend/internal/platform/db"
	"github.com/oops-reader/oops-reader-backend/internal/platform/log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}

	logger := log.NewLogger(cfg)
	logger.Info("Starting Oops Reader Backend Worker")

	dbPool, err := db.NewMySQL(cfg)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		logger.Info("Worker started")
		<-ctx.Done()
		logger.Info("Worker shutting down...")
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Received shutdown signal")

	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer timeoutCancel()

	cancel()

	select {
	case <-timeoutCtx.Done():
		logger.Warn("Worker shutdown timeout")
	default:
		logger.Info("Worker shutdown complete")
	}
}
