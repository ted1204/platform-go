package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/linskybing/platform-go/internal/config"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/pkg/types"
	"github.com/linskybing/platform-go/pkg/utils"
)

var jwtKey []byte

// Init sets the JWT signing key.
func Init() {
	jwtKey = []byte(config.JwtSecret)
}

// GenerateToken issues a signed token and reports admin status.
var GenerateToken = func(userID uint, username string, expireDuration time.Duration, repos repository.UserGroupRepo) (string, bool, error) {
	isAdmin, err := utils.IsSuperAdmin(userID, repos)
	if err != nil {
		return "", false, err
	}
	claims := &types.Claims{
		UserID:   userID,
		Username: username,
		IsAdmin:  isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expireDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    config.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(jwtKey)
	if err != nil {
		return "", false, err
	}

	return signedToken, isAdmin, nil
}

// ParseToken validates and extracts claims.
func ParseToken(tokenStr string) (*types.Claims, error) {
	claims := &types.Claims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil || !token.Valid {
		return nil, err
	}

	return claims, nil
}

// JWTAuthMiddleware validates Bearer token in Authorization header or cookie.
func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenStr string

		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be Bearer {token}"})
				c.Abort()
				return
			}
			tokenStr = parts[1]
		} else {
			if cookie, err := c.Cookie("token"); err == nil {
				tokenStr = cookie
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization required (header or cookie)"})
				c.Abort()
				return
			}
		}

		claims, err := ParseToken(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: " + err.Error()})
			c.Abort()
			return
		}

		// Explicitly enforce expiration to avoid lax parser behavior
		if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
			c.Abort()
			return
		}

		c.Set("claims", claims)
		c.Next()
	}
}
