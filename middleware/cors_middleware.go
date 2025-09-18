package middleware

import (
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	config := cors.Config{
		AllowOriginFunc: func(origin string) bool {
			if strings.HasPrefix(origin, "http://localhost:") {
				return true
			}
			if strings.HasPrefix(origin, "http://10.121.124.22:") {
				return true
			}
			if strings.HasPrefix(origin, "http://223.137.82.130:") {
				return true
			}
			return false
		},
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
