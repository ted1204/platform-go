package middleware

import (
	"errors"
	"net/http"
	"reflect"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/repositories"
	"github.com/linskybing/platform-go/src/response"
	"github.com/linskybing/platform-go/src/types"
	"github.com/linskybing/platform-go/src/utils"
)

type Auth struct {
	repos *repositories.Repos
}

func NewAuth(repos *repositories.Repos) *Auth {
	return &Auth{repos: repos}
}

// --- Extractors ---

type GIDExtractor func(c *gin.Context, repos *repositories.Repos) (uint, error)

func FromPayload(dtoType any) GIDExtractor {
	return func(c *gin.Context, repos *repositories.Repos) (uint, error) {
		// Dynamically create a new DTO instance
		dtoValue := reflect.New(reflect.TypeOf(dtoType)).Interface()

		// Bind form data directly
		if err := c.ShouldBind(dtoValue); err != nil {
			return 0, err
		}

		if getter, ok := dtoValue.(dto.GIDGetter); ok {
			return getter.GetGID(), nil
		}
		if getter, ok := dtoValue.(dto.GIDByRepoGetter); ok {
			return getter.GetGIDByRepo(repos), nil
		}
		return 0, errors.New("DTO does not implement GIDGetter or GIDByRepoGetter")
	}
}

func FromIDParam(lookup func(uint) (uint, error)) GIDExtractor {
	return func(c *gin.Context, repos *repositories.Repos) (uint, error) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			return 0, err
		}
		return lookup(id)
	}
}

// --- Middleware Methods ---

func (a *Auth) Admin() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := c.MustGet("claims").(*types.Claims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		isAdmin, err := utils.IsSuperAdmin(claims.UserID, a.repos.View)
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

		isAdmin, err := utils.IsSuperAdmin(currentUID, a.repos.View)
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

func (a *Auth) GroupMember(extractor GIDExtractor) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		uid, err := utils.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Unauthorized"})
			c.Abort()
			return
		}

		permitted, err := utils.CheckGroupPermission(uid, gid, a.repos.View)
		if err != nil || !permitted {
			c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "Permission denied for this group"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (a *Auth) GroupAdmin(extractor GIDExtractor) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		uid, err := utils.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Unauthorized"})
			c.Abort()
			return
		}

		permitted, err := utils.CheckGroupAdminPermission(uid, gid, a.repos.View)
		if err != nil || !permitted {
			c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "Permission denied for this group"})
			c.Abort()
			return
		}

		c.Next()
	}
}
