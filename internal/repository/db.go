package repository

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config describes the database connection.
type Config struct {
	DSN          string
	LogLevel     string    // silent, error, warn or info (default: info, every SQL statement is logged)
	MaxOpenConns int       // default 25
	MaxIdleConns int       // default 5
	LogOutput    io.Writer // where the SQL log goes (default os.Stdout); tests capture it
}

const (
	openAttempts = 10
	openRetryGap = time.Second
)

// Open connects to PostgreSQL through GORM and configures the connection pool.
// The database may still be starting (docker compose, CI service container), so the
// connection is retried for a few seconds. Every SQL statement is logged on stdout at
// the info level: that is how the N+1 problem is observed and how CI counts SELECTs.
func Open(cfg Config) (*gorm.DB, error) {
	out := cfg.LogOutput
	if out == nil {
		out = os.Stdout
	}
	gormLogger := logger.New(log.New(out, "", log.LstdFlags), logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  parseLogLevel(cfg.LogLevel),
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true, // log $1, $2 ... never the bound values (hashes, e-mails)
		Colorful:                  false,
	})
	gormConfig := &gorm.Config{
		Logger:         gormLogger,
		TranslateError: true, // driver errors become gorm.ErrDuplicatedKey etc., see errors.go
		NowFunc:        func() time.Time { return time.Now().UTC() },
	}

	var (
		db  *gorm.DB
		err error
	)
	for attempt := 1; attempt <= openAttempts; attempt++ {
		db, err = gorm.Open(postgres.Open(cfg.DSN), gormConfig)
		if err == nil {
			break
		}
		if attempt < openAttempts {
			log.Printf("database not ready (attempt %d/%d): %v", attempt, openAttempts, err)
			time.Sleep(openRetryGap)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("database pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(withDefault(cfg.MaxOpenConns, 25))
	sqlDB.SetMaxIdleConns(withDefault(cfg.MaxIdleConns, 5))
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	return db, nil
}

func withDefault(value, def int) int {
	if value <= 0 {
		return def
	}
	return value
}

func parseLogLevel(level string) logger.LogLevel {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "warn", "warning":
		return logger.Warn
	default:
		return logger.Info
	}
}
