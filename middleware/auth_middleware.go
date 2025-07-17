package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/utils"
)

func AuthorizeUserOrAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := c.MustGet("claims").(*Claims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		currentUID := claims.UserID
		idParam := c.Param("id")
		if idParam == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Missing user id"})
			return
		}

		targetUID64, err := strconv.ParseUint(idParam, 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid user id"})
			return
		}
		targetUID := uint(targetUID64)

		if currentUID == targetUID {
			c.Next()
			return
		}

		isAdmin, err := utils.IsSuperAdmin(currentUID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		if !isAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
			return
		}

		c.Next()
	}
}

func AuthorizeAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := c.MustGet("claims").(*Claims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		uid := claims.UserID

		isAdmin, err := utils.IsSuperAdmin(uid)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if !isAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin only"})
			return
		}

		c.Next()
	}
}
