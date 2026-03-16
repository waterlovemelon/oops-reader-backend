package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/oops-reader/oops-reader-backend/internal/platform/config"
)

func NewMySQL(cfg *config.Config) (*sql.DB, error) {
	mysqlConfig := mysql.Config{
		User:                 cfg.Database.User,
		Passwd:               cfg.Database.Password,
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%d", cfg.Database.Host, cfg.Database.Port),
		DBName:               cfg.Database.Database,
		ParseTime:            true,
		Loc:                  time.Local,
		AllowNativePasswords: true,
		Params: map[string]string{
			"charset":   "utf8mb4",
			"parseTime": "True",
			"loc":       "Local",
		},
	}

	dsn := mysqlConfig.FormatDSN()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.Database.ConnMaxIdleTime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
