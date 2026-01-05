package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/application"
	"github.com/linskybing/platform-go/internal/domain/group"
	"github.com/linskybing/platform-go/pkg/response"
	"github.com/linskybing/platform-go/pkg/utils"
)

type UserGroupHandler struct {
	svc *application.UserGroupService
}

func NewUserGroupHandler(svc *application.UserGroupService) *UserGroupHandler {
	return &UserGroupHandler{svc: svc}
}

// @Summary Get a user-group relation by user ID and group ID
// @Tags user_group
// @Produce json
// @Param u_id query uint true "User ID"
// @Param g_id query uint true "Group ID"
// @Success 200 {object} response.SuccessResponse{data=group.UserGroupView}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /user-group [get]
func (h *UserGroupHandler) GetUserGroup(c *gin.Context) {
	uidStr := c.Query("u_id")
	gidStr := c.Query("g_id")

	if uidStr == "" || gidStr == "" {
		c.JSON(http.StatusOK, []group.UserGroup{})
		return
	}

	uid, err := strconv.ParseUint(uidStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid u_id"})
		return
	}
	gid, err := strconv.ParseUint(gidStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid g_id"})
		return
	}

	userGroup, err := h.svc.GetUserGroup(uint(uid), uint(gid))
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "User-Group relation not found"})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Code:    0,
		Message: "success",
		Data:    userGroup,
	})
}

// @Summary Get all users in a group
// @Tags user_group
// @Produce json
// @Param g_id query uint true "Group ID"
// @Success 200 {object} response.SuccessResponse{data=[]group.GroupUsers}
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /user-group/by-group [get]
func (h *UserGroupHandler) GetUserGroupsByGID(c *gin.Context) {
	gidStr := c.Query("g_id")
	if gidStr == "" {
		gidStr = c.Query("gid")
	}
	if gidStr == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Missing g_id"})
		return
	}
	gid, err := strconv.ParseUint(gidStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid g_id"})
		return
	}

	rawData, err := h.svc.GetUserGroupsByGID(uint(gid))
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, rawData)
}

// controller
// @Summary Get all groups for a user
// @Tags user_group
// @Produce json
// @Param u_id query uint true "User ID"
// @Success 200 {object} response.SuccessResponse{data=[]group.UserGroups}
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /user-group/by-user [get]
func (h *UserGroupHandler) GetUserGroupsByUID(c *gin.Context) {
	uidStr := c.Query("u_id")
	if uidStr == "" {
		uidStr = c.Query("uid")
	}
	if uidStr == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Missing u_id"})
		return
	}
	uid, err := strconv.ParseUint(uidStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid u_id"})
		return
	}

	rawData, err := h.svc.GetUserGroupsByUID(uint(uid))
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, rawData)
}

// @Summary Create a user-group relation
// @Tags user_group
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param u_id formData uint true "User ID"
// @Param g_id formData uint true "Group ID"
// @Param role formData string true "Role (admin, manager, user)"
// @Success 201 {object} group.UserGroup
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /user-group [post]
func (h *UserGroupHandler) CreateUserGroup(c *gin.Context) {
	var input group.UserGroupInputDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	requesterID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Unauthorized"})
		return
	}

	if input.Role == "admin" {
		// Only super admin can elevate to admin role.
		isSuper, superErr := utils.IsSuperAdmin(requesterID, h.svc.Repos.View)
		if superErr != nil {
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "internal error"})
			return
		}
		if !isSuper {
			c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "admin role assignment requires super admin"})
			return
		}
	}

	userGroup := &group.UserGroup{
		UID:  input.UID,
		GID:  input.GID,
		Role: input.Role,
	}

	if _, err := h.svc.CreateUserGroup(c, userGroup); err != nil {
		c.JSON(http.StatusOK, userGroup)
		return
	}
	c.JSON(http.StatusOK, userGroup)
}

// @Summary Update role of a user-group relation
// @Tags user_group
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param u_id formData uint true "User ID"
// @Param g_id formData uint true "Group ID"
// @Param role formData string true "Role (admin, manager, user)"
// @Success 200 {object} group.UserGroup
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /user-group [put]
func (h *UserGroupHandler) UpdateUserGroup(c *gin.Context) {
	var input group.UserGroupInputDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	requesterID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Unauthorized"})
		return
	}

	if input.Role == "admin" {
		isSuper, superErr := utils.IsSuperAdmin(requesterID, h.svc.Repos.View)
		if superErr != nil {
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "internal error"})
			return
		}
		if !isSuper {
			c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "admin role assignment requires super admin"})
			return
		}
	}

	existing, err := h.svc.GetUserGroup(input.UID, input.GID)
	if err != nil {
		// If relation does not exist, create it instead of failing the update
		created := &group.UserGroup{UID: input.UID, GID: input.GID, Role: input.Role}
		if _, errCreate := h.svc.CreateUserGroup(c, created); errCreate != nil {
			c.JSON(http.StatusOK, created)
			return
		}
		c.JSON(http.StatusOK, created)
		return
	}

	updated := &group.UserGroup{
		UID:  existing.UID,
		GID:  existing.GID,
		Role: input.Role,
	}

	if _, err := h.svc.UpdateUserGroup(c, updated, existing); err != nil {
		c.JSON(http.StatusOK, updated)
		return
	}

	c.JSON(http.StatusOK, updated)
}

// @Summary Delete a user-group relation
// @Tags user_group
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param u_id formData uint true "User ID"
// @Param g_id formData uint true "Group ID"
// @Success 204 {string} string "deleted"
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /user-group [delete]
func (h *UserGroupHandler) DeleteUserGroup(c *gin.Context) {
	var input group.UserGroupDeleteDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.svc.DeleteUserGroup(c, input.UID, input.GID); err != nil {
		if err == application.ErrReservedUser {
			c.JSON(http.StatusForbidden, response.ErrorResponse{Error: err.Error()})
		} else {
			c.JSON(http.StatusNotFound, response.ErrorResponse{Error: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, response.MessageResponse{Message: "deleted"})
}
