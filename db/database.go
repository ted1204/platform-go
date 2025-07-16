package db

import (
    "log"
    "os"
    "platform-go/models"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

var DB *gorm.DB

func Init() {
    dsn := os.Getenv("DB_DSN")
    var err error
    DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        log.Fatal("Failed to connect to DB:", err)
    }

    DB.AutoMigrate(&models.User{})
}