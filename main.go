package main

import (
    "github.com/joho/godotenv"
    "log"
    "platform-go/config"
    "platform-go/db"
    "platform-go/routes"
)

func main() {
    err := godotenv.Load()
    if err != nil {
        log.Fatal("Error loading .env file")
    }

    db.Init()

    router := routes.SetupRouter()
    port := config.GetEnv("PORT", "8080")
    router.Run(":" + port)
}