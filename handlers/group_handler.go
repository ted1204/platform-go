package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/models"
	"github.com/linskybing/platform-go/response"
	"github.com/linskybing/platform-go/utils"
)

// GetGroups godoc
// @Summary List all groups
// @Tags groups
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Group
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /groups [get]
func GetGroups(c *gin.Context) {
	var groups []models.Group
	if err := db.DB.Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}

// GetGroupByID godoc
// @Summary Get group by ID
// @Tags groups
// @Security BearerAuth
// @Produce json
// @Param id path int true "Group ID"
// @Success 200 {object} models.Group
// @Failure 400 {object} response.ErrorResponse "Invalid group id"
// @Failure 404 {object} response.ErrorResponse "Group not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /groups/{id} [get]
func GetGroupByID(c *gin.Context) {
	id := c.Param("id")
	gid, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid group id"})
		return
	}

	gidUint := uint(gid)

	var group models.Group
	if err := db.DB.First(&group, gidUint).Error; err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "group not found"})
		return
	}
	c.JSON(http.StatusOK, group)
}

// CreateGroup godoc
// @Summary Create a new group
// @Tags groups
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param group_name formData string true "Group name"
// @Param description formData string false "Description"
// @Success 201 {object} response.GroupResponse
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 403 {object} response.ErrorResponse "Forbidden (reserved name)"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /groups [post]
func CreateGroup(c *gin.Context) {

	var input dto.GroupCreateDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	group := models.Group{
		GroupName: input.GroupName,
	}

	if input.GroupName == "super" {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "cannot create 'super' group"})
		return
	}

	if input.Description != nil {
		group.Description = *input.Description
	}

	if err := db.DB.Create(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	// audit log
	userid, _ := utils.GetUserIDFromContext(c)
	if err := utils.LogAudit(c, userid, "create", "group", group.GID, nil, group, nil); err != nil {
		log.Printf("Audit log failed: %v", err)
	}

	c.JSON(http.StatusCreated, response.GroupResponse{
		Message: "group created",
		Group:   group,
	})
}

// UpdateGroup godoc
// @Summary Update group by ID
// @Tags groups
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "Group ID"
// @Param input body dto.GroupUpdateDTO true "Group update input"
// @Success 200 {object} models.Group
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 404 {object} response.ErrorResponse "Group not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /groups/{id} [put]
func UpdateGroup(c *gin.Context) {
	id := c.Param("id")
	gid, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid group id"})
		return
	}
	gidUint := uint(gid)

	var input dto.GroupUpdateDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	var group models.Group
	if err := db.DB.First(&group, gidUint).Error; err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "group not found"})
		return
	}

	// deepcopy
	oldGroup := group

	if input.GroupName != nil {
		if *input.GroupName == "super" {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Cannot use reserved group name 'super'"})
			return
		}
		group.GroupName = *input.GroupName
	}
	if input.Description != nil {
		group.Description = *input.Description
	}

	if err := db.DB.Save(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	// audit log
	userid, _ := utils.GetUserIDFromContext(c)
	if err := utils.LogAudit(c, userid, "update", "group", group.GID, oldGroup, group, nil); err != nil {
		log.Printf("Audit log failed: %v", err)
	}

	c.JSON(http.StatusOK, group)
}

// DeleteGroup godoc
// @Summary Delete group by ID
// @Tags groups
// @Security BearerAuth
// @Produce json
// @Param id path int true "Group ID"
// @Success 200 {object} response.MessageResponse "Group deleted"
// @Failure 400 {object} response.ErrorResponse "Invalid group id"
// @Failure 403 {object} response.ErrorResponse "Forbidden to delete 'super' group"
// @Failure 404 {object} response.ErrorResponse "Group not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /groups/{id} [delete]
func DeleteGroup(c *gin.Context) {
	id := c.Param("id")
	gid, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid group id"})
		return
	}
	gidUint := uint(gid)

	var group models.Group
	err = db.DB.Select("group_name").First(&group, gidUint).Error
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "group not found"})
		return
	}
	if group.GroupName == "super" {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "cannot delete super group"})
		return
	}

	// audit log
	userid, _ := utils.GetUserIDFromContext(c)
	if err := utils.LogAudit(c, userid, "update", "group", group.GID, group, nil, nil); err != nil {
		log.Printf("Audit log failed: %v", err)
	}

	if err := db.DB.Delete(&models.Group{}, gidUint).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.MessageResponse{Message: "group deleted"})
}
