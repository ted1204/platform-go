package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/models"
	"github.com/linskybing/platform-go/response"
	"github.com/linskybing/platform-go/services"
)

type UserGroupHandler struct {
	svc *services.UserGroupService
}

func NewUserGroupHandler(svc *services.UserGroupService) *UserGroupHandler {
	return &UserGroupHandler{svc: svc}
}

// @Summary Get a user-group relation by user ID and group ID
// @Tags user_group
// @Produce json
// @Param u_id query uint true "User ID"
// @Param g_id query uint true "Group ID"
// @Success 200 {object} response.SuccessResponse{data=models.UserGroupView}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /user-group [get]
func (h *UserGroupHandler) GetUserGroup(c *gin.Context) {
	uidStr := c.Query("u_id")
	gidStr := c.Query("g_id")

	if uidStr == "" || gidStr == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Missing u_id or g_id"})
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
// @Success 200 {object} response.SuccessResponse{data=[]models.GroupUsers}
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /user-group/by-group [get]
func (h *UserGroupHandler) GetUserGroupsByGID(c *gin.Context) {
	gidStr := c.Query("g_id")
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
	userGroups := h.svc.FormatByGID(rawData)
	c.JSON(http.StatusOK, response.SuccessResponse{
		Code:    0,
		Message: "success",
		Data:    userGroups,
	})
}

// controller
// @Summary Get all groups for a user
// @Tags user_group
// @Produce json
// @Param u_id query uint true "User ID"
// @Success 200 {object} response.SuccessResponse{data=[]dto.UserGroups}
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /user-group/by-user [get]
func (h *UserGroupHandler) GetUserGroupsByUID(c *gin.Context) {
	uidStr := c.Query("u_id")
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
	userGroups := h.svc.FormatByUID(rawData)

	c.JSON(http.StatusOK, response.SuccessResponse{
		Code:    0,
		Message: "success",
		Data:    userGroups,
	})
}

// @Summary Create a user-group relation
// @Tags user_group
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param u_id formData uint true "User ID"
// @Param g_id formData uint true "Group ID"
// @Param role formData string true "Role (admin, manager, user)"
// @Success 201 {object} models.UserGroup
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /user-group [post]
func (h *UserGroupHandler) CreateUserGroup(c *gin.Context) {
	var input dto.UserGroupInputDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	userGroup := &models.UserGroup{
		UID:  input.UID,
		GID:  input.GID,
		Role: input.Role,
	}

	if _, err := h.svc.CreateUserGroup(c, userGroup); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, userGroup)
}

// @Summary Update role of a user-group relation
// @Tags user_group
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param u_id formData uint true "User ID"
// @Param g_id formData uint true "Group ID"
// @Param role formData string true "Role (admin, manager, user)"
// @Success 200 {object} models.UserGroup
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /user-group [put]
func (h *UserGroupHandler) UpdateUserGroup(c *gin.Context) {
	var input dto.UserGroupInputDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	existing, err := h.svc.GetUserGroup(input.UID, input.GID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "User-Group relation not found"})
		return
	}

	updated := &models.UserGroup{
		UID:  existing.UID,
		GID:  existing.GID,
		Role: input.Role,
	}

	if _, err := h.svc.UpdateUserGroup(c, updated, existing); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
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
	uidStr := c.Query("u_id")
	gidStr := c.Query("g_id")

	if uidStr == "" || gidStr == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Missing u_id or g_id"})
		return
	}

	uid64, err := strconv.ParseUint(uidStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid u_id"})
		return
	}
	gid64, err := strconv.ParseUint(gidStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid g_id"})
		return
	}

	if err := h.svc.DeleteUserGroup(c, uint(uid64), uint(gid64)); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
