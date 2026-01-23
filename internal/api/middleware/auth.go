package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/pkg/response"
	"github.com/linskybing/platform-go/pkg/types"
	"github.com/linskybing/platform-go/pkg/utils"
)

// Auth handles authorization middleware
type Auth struct {
	repos *repository.Repos
}

// NewAuth creates a new Auth middleware instance
func NewAuth(repos *repository.Repos) *Auth {
	return &Auth{repos: repos}
}

// --- Extractors ---

// GIDExtractor extracts group ID from context
type GIDExtractor func(c *gin.Context, repos *repository.Repos) (uint, error)

// FromPayload creates an extractor that gets group ID from request payload
func FromPayload(dtoType any) GIDExtractor {
	return func(c *gin.Context, repos *repository.Repos) (uint, error) {
		// Dynamically create a new DTO instance
		dtoValue := reflect.New(reflect.TypeOf(dtoType)).Interface()

		// Read and preserve the raw body for downstream handlers.
		bodyBytes, err := c.GetRawData()
		if err != nil {
			return 0, err
		}

		// Attempt JSON unmarshal first.
		if len(bodyBytes) > 0 {
			if err := json.Unmarshal(bodyBytes, dtoValue); err != nil {
				// Fall back to the default binder (e.g., form data)
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				if err := c.ShouldBind(dtoValue); err != nil {
					return 0, err
				}
			}
		} else {
			if err := c.ShouldBind(dtoValue); err != nil {
				return 0, err
			}
		}

		// Restore body so the handler can bind again.
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		if getter, ok := dtoValue.(project.GIDGetter); ok {
			return getter.GetGID(), nil
		}
		return 0, errors.New("DTO does not implement GIDGetter")
	}
}

// FromProjectIDInPayload creates an extractor that gets GID from ProjectID in payload
func FromProjectIDInPayload(dtoType any) GIDExtractor {
	return func(c *gin.Context, repos *repository.Repos) (uint, error) {
		// Dynamically create a new DTO instance
		dtoValue := reflect.New(reflect.TypeOf(dtoType)).Interface()

		// Read and preserve the raw body for downstream handlers.
		bodyBytes, err := c.GetRawData()
		if err != nil {
			return 0, err
		}

		// Restore body for form data binding
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Bind form data directly
		if err := c.ShouldBind(dtoValue); err != nil {
			return 0, err
		}

		// Restore body again so the handler can bind again.
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Check if DTO has GetProjectID method
		type ProjectIDGetter interface {
			GetProjectID() uint
		}

		if getter, ok := dtoValue.(ProjectIDGetter); ok {
			projectID := getter.GetProjectID()
			// Get GID from ProjectID
			return repos.Project.GetGroupIDByProjectID(projectID)
		}
		return 0, errors.New("DTO does not implement GetProjectID")
	}
}

// FromIDParam creates an extractor that gets group ID from URL parameter
func FromIDParam(lookup func(uint) (uint, error)) GIDExtractor {
	return func(c *gin.Context, repos *repository.Repos) (uint, error) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			return 0, err
		}
		return lookup(id)
	}
}

// FromProjectIDInParam creates an extractor that gets GID from project_id URL parameter
func FromProjectIDInParam() GIDExtractor {
	return func(c *gin.Context, repos *repository.Repos) (uint, error) {
		projectIDStr := c.Param("project_id")
		projectID, err := strconv.ParseUint(projectIDStr, 10, 32)
		if err != nil {
			return 0, errors.New("invalid project_id parameter")
		}
		return repos.Project.GetGroupIDByProjectID(uint(projectID))
	}
}

// --- Middleware Methods ---

// Admin checks if user is a super admin
func (a *Auth) Admin() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := c.MustGet("claims").(*types.Claims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		isAdmin, err := utils.IsSuperAdmin(claims.UserID, a.repos.UserGroup)
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

// UserOrAdmin checks if user is the target user or a super admin
func (a *Auth) UserOrAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := c.MustGet("claims").(*types.Claims)
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

		isAdmin, err := utils.IsSuperAdmin(currentUID, a.repos.UserGroup)
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

// GroupMember checks if user is a member of the group
func (a *Auth) GroupMember(extractor GIDExtractor) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, err := utils.GetUserIDFromContext(c)
		if err == nil && uid == 1 {
			c.Next()
			return
		}

		gid, err := extractor(c, a.repos)
		if err != nil {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid input: " + err.Error()})
			c.Abort()
			return
		}

		if gid == 0 {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Group ID cannot be zero"})
			c.Abort()
			return
		}

		uid, err = utils.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Unauthorized"})
			c.Abort()
			return
		}

		permitted, err := utils.CheckGroupPermission(uid, gid, a.repos.UserGroup)
		if err != nil || !permitted {
			c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "Permission denied for this group"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GroupMember checks if user is a manager or admin of the group
func (a *Auth) GroupManager(extractor GIDExtractor) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, err := utils.GetUserIDFromContext(c)
		if err == nil && uid == 1 {
			c.Next()
			return
		}

		gid, err := extractor(c, a.repos)
		if err != nil {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid input: " + err.Error()})
			c.Abort()
			return
		}

		if gid == 0 {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Group ID cannot be zero"})
			c.Abort()
			return
		}

		uid, err = utils.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Unauthorized"})
			c.Abort()
			return
		}

		permitted, err := utils.CheckGroupManagePermission(uid, gid, a.repos.UserGroup)
		if err != nil || !permitted {
			c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "Permission denied for this group"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GroupAdmin checks if user is an admin of the group
func (a *Auth) GroupAdmin(extractor GIDExtractor) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, err := utils.GetUserIDFromContext(c)
		if err == nil && uid == 1 {
			c.Next()
			return
		}

		gid, err := extractor(c, a.repos)
		if err != nil {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid input: " + err.Error()})
			c.Abort()
			return
		}

		if gid == 0 {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Group ID cannot be zero"})
			c.Abort()
			return
		}

		uid, err = utils.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Unauthorized"})
			c.Abort()
			return
		}

		permitted, err := utils.CheckGroupAdminPermission(uid, gid, a.repos.UserGroup)
		if err != nil || !permitted {
			c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "Permission denied for this group"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AuthMiddleware validates request (placeholder for future auth)
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// LoggingMiddleware logs requests (placeholder; hook for real logging)
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// CORSMiddleware matches the legacy behavior from src/middleware/cors_middleware.go
func CORSMiddleware() gin.HandlerFunc {
	config := cors.Config{
		AllowOriginFunc: func(origin string) bool {
			if strings.HasPrefix(origin, "http://localhost:") {
				return true
			}
			if strings.HasPrefix(origin, "http://10.121.124.21:") {
				return true
			}
			if strings.HasPrefix(origin, "http://10.121.124.22:") {
				return true
			}
			if strings.HasPrefix(origin, "http://223.137.82.130:") {
				return true
			}
			if strings.HasPrefix(origin, "http://127.0.0.1:") {
				return true
			}
			if strings.HasPrefix(origin, "http://192.168.109.1:") {
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
		if strings.EqualFold(upgrade, "websocket") {
			c.Next()
			return
		}
		corsHandler(c)
	}
}
