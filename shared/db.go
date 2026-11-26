package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/KevinZonda/Panel/models/dbmod"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func initDb() error {
	if cfg == nil {
		return fmt.Errorf("config is not initialized")
	}

	if dir := sqliteDir(cfg.DatabaseDSN); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create sqlite directory: %w", err)
		}
	}

	database, err := gorm.Open(sqlite.Open(cfg.DatabaseDSN), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		return fmt.Errorf("get sqlite connection: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := database.AutoMigrate(&dbmod.User{}, &dbmod.AuthSession{}); err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("migrate sqlite: %w", err)
	}

	db = database
	return nil
}

func sqliteDir(dsn string) string {
	path := dsn
	if strings.HasPrefix(path, "file:") {
		path = strings.TrimPrefix(path, "file:")
	}
	if i := strings.Index(path, "?"); i >= 0 {
		path = path[:i]
	}
	if path == "" || path == ":memory:" || strings.Contains(dsn, "mode=memory") {
		return ""
	}
	return filepath.Dir(path)
}

func CloseDB() error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
