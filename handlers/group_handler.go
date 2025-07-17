package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/models"
)

// Get /group
func GetGroups(c *gin.Context) {
	var groups []models.Group
	if err := db.DB.Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}

// Get /group/:id
func GetGroupByID(c *gin.Context) {
	id := c.Param("id")
	gid, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	gidUint := uint(gid)

	var group models.Group
	if err := db.DB.First(&group, gidUint).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}
	c.JSON(http.StatusOK, group)
}

// POST /group
func CreateGroup(c *gin.Context) {
	var input dto.GroupCreateDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group := models.Group{
		GroupName: input.GroupName,
	}

	if input.GroupName == "super" {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot create 'super' group"})
		return
	}

	if input.Description != nil {
		group.Description = *input.Description
	}

	if err := db.DB.Create(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "group created",
		"group":   group,
	})
}

// PUT /group
func UpdateGroup(c *gin.Context) {
	id := c.Param("id")
	gid, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}
	gidUint := uint(gid)

	var input dto.GroupUpdateDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var group models.Group
	if err := db.DB.First(&group, gidUint).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}

	if input.GroupName != nil {
		if *input.GroupName == "super" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot use reserved group name 'super'"})
			return
		}
		group.GroupName = *input.GroupName
	}
	if input.Description != nil {
		group.Description = *input.Description
	}

	if err := db.DB.Save(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, group)
}

// DELETE /group/:id
func DeleteGroup(c *gin.Context) {
	id := c.Param("id")
	gid, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}
	gidUint := uint(gid)

	var group models.Group
	err = db.DB.Select("group_name").First(&group, gidUint).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "group not found"})
		return
	}
	if group.GroupName == "super" {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete super group"})
		return
	}

	if err := db.DB.Delete(&models.Group{}, gidUint).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "group deleted"})
}
