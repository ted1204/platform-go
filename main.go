// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Authorization header using the Bearer scheme. Example: "Bearer {token}"
package main

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/config"
	"github.com/linskybing/platform-go/db"
	_ "github.com/linskybing/platform-go/docs"
	"github.com/linskybing/platform-go/middleware"
	"github.com/linskybing/platform-go/minio"
	"github.com/linskybing/platform-go/routes"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	config.LoadConfig()
	db.Init()
	minio.InitMinio()
	middleware.Init()

	r := gin.Default()
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.SetTrustedProxies([]string{"127.0.0.1"})
	routes.RegisterRoutes(r)
	addr := fmt.Sprintf(":%s", config.ServerPort)
	r.Run(addr)
}
