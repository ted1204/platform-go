package main

import (
    "github.com/gin-gonic/gin"
    "platform-go/db"
    "platform-go/routes"
)

func main() {
    db.Init()
    r := gin.Default()
    routes.RegisterProjectRoutes(r)
    r.Run(":8080")
}
