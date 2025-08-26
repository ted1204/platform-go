package db

import (
	"fmt"
	"log"

	"github.com/linskybing/platform-go/config"
	"github.com/linskybing/platform-go/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init() {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DbHost,
		config.DbPort,
		config.DbUser,
		config.DbPassword,
		config.DbName,
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to DB:", err)
	}

	if err := DB.AutoMigrate(&models.User{}); err != nil {
		log.Fatal("Failed to auto migrate:", err)
	}

	log.Println("Database connected and migrated")
}

func InitWithGormDB(gormDB *gorm.DB) {
	DB = gormDB
}
