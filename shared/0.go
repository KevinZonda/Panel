package shared

import (
	"fmt"

	"github.com/KevinZonda/Panel/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	cfg    *models.Config
	db     *gorm.DB
	engine *gin.Engine
)

func InitSvcs() error {
	if err := initDb(); err != nil {
		return err
	}
	if err := initGin(); err != nil {
		return err
	}
	return nil
}

func GetConfig() (*models.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is not initialized")
	}
	return cfg, nil
}

func GetDB() (*gorm.DB, error) {
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	return db, nil
}

func GetGinEngine() (*gin.Engine, error) {
	if engine == nil {
		return nil, fmt.Errorf("gin engine is not initialized")
	}
	return engine, nil
}
