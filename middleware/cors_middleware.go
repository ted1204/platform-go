package middleware

import (
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	config := cors.Config{
		AllowOrigins:     []string{"http://10.121.124.22:5173", "http://223.137.82.130:5173", "http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	corsHandler := cors.New(config)
	return func(c *gin.Context) {
		upgrade := c.GetHeader("Upgrade")
		if strings.ToLower(upgrade) == "websocket" {
			c.Next()
			return
		}
		corsHandler(c)
	}
}
