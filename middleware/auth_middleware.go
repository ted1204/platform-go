package middleware

import (
	"net/http"
	"reflect"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/repositories"
	"github.com/linskybing/platform-go/response"
	"github.com/linskybing/platform-go/types"
	"github.com/linskybing/platform-go/utils"
)

func AuthorizeUserOrAdmin(repos repositories.ViewRepo) gin.HandlerFunc {
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

		isAdmin, err := utils.IsSuperAdmin(currentUID, repos)
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

func AuthorizeAdmin(repos repositories.ViewRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := c.MustGet("claims").(*types.Claims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		uid := claims.UserID

		isAdmin, err := utils.IsSuperAdmin(uid, repos)
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

func CheckPermissionPayload(permission string, dtoType any, repos repositories.ViewRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Dynamically create a new DTO instance (must implement GIDGetter)
		dtoValue := reflect.New(reflect.TypeOf(dtoType)).Interface()

		// Bind form data directly (supports x-www-form-urlencoded, multipart/form-data)
		if err := c.ShouldBind(dtoValue); err != nil {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid input: " + err.Error()})
			c.Abort()
			return
		}

		gidGetter, ok := dtoValue.(dto.GIDGetter)
		if !ok {
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "DTO does not implement GIDGetter"})
			c.Abort()
			return
		}

		gid := gidGetter.GetGID()
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

		permitted, err := utils.CheckGroupPermission(uid, gid, repos)
		if err != nil || !permitted {
			c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "Permission denied for this group"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func CheckStrictPermissionPayload(permission string, dtoType any, repos repositories.ViewRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Dynamically create a new DTO instance (must implement GIDGetter)
		dtoValue := reflect.New(reflect.TypeOf(dtoType)).Interface()

		// Bind form data directly (supports x-www-form-urlencoded, multipart/form-data)
		if err := c.ShouldBind(dtoValue); err != nil {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid input: " + err.Error()})
			c.Abort()
			return
		}

		gidGetter, ok := dtoValue.(dto.GIDGetter)
		if !ok {
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "DTO does not implement GIDGetter"})
			c.Abort()
			return
		}

		gid := gidGetter.GetGID()
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

		permitted, err := utils.CheckGroupAdminPermission(uid, gid, repos)
		if err != nil || !permitted {
			c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "Permission denied for this group"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func CheckPermissionPayloadByRepo(permission string, dtoType any, repos *repositories.Repos) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Dynamically create a new DTO instance (must implement GIDGetter)
		dtoValue := reflect.New(reflect.TypeOf(dtoType)).Interface()

		// Bind form data directly (supports x-www-form-urlencoded, multipart/form-data)
		if err := c.ShouldBind(dtoValue); err != nil {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid input: " + err.Error()})
			c.Abort()
			return
		}

		gidGetter, ok := dtoValue.(dto.GIDByRepoGetter)
		if !ok {
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "DTO does not implement GIDGetter"})
			c.Abort()
			return
		}

		gid := gidGetter.GetGIDByRepo(repos)
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

		permitted, err := utils.CheckGroupPermission(uid, gid, repos.View)
		if err != nil || !permitted {
			c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "Permission denied for this group"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func CheckPermissionByParam(getDataByID func(uint) (uint, error), repos repositories.ViewRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid group id"})
			return
		}

		// Lookup GID from the given resource ID using the passed function
		gid, err := getDataByID(id)
		if err != nil {
			c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "Resource not found"})
			c.Abort()
			return
		}

		// Get current user ID
		uid, err := utils.GetUserIDFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Unauthorized"})
			c.Abort()
			return
		}

		// Check permission using the resolved GID
		permitted, err := utils.CheckGroupPermission(uid, gid, repos)
		if err != nil || !permitted {
			c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "Permission denied for this group"})
			c.Abort()
			return
		}

		c.Next()
	}
}
