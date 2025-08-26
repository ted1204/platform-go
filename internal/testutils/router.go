package testutils

import (
	"github.com/linskybing/platform-go/routes"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	routes.RegisterRoutes(r)
	return r
}
