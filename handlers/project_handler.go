package handlers

import (
    "net/http"
    "strconv"

    "github.com/gin-gonic/gin"
    "platform-go/models"
    "platform-go/db"
)

// GET /projects
func GetProjects(c *gin.Context) {
    var projects []models.Project
    if err := db.DB.Find(&projects).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, projects)
}

// GET /projects/:id
func GetProjectByID(c *gin.Context) {
    id := c.Param("id")
    var project models.Project

    if err := db.DB.First(&project, id).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
        return
    }
    c.JSON(http.StatusOK, project)
}

// POST /projects
func CreateProject(c *gin.Context) {
    var project models.Project
    if err := c.ShouldBindJSON(&project); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if err := db.DB.Create(&project).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusCreated, project)
}

// PUT /projects/:id
func UpdateProject(c *gin.Context) {
    id := c.Param("id")
    var project models.Project

    if err := db.DB.First(&project, id).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
        return
    }

    var input models.Project
    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    project.ProjectName = input.ProjectName
    project.GID = input.GID

    if err := db.DB.Save(&project).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, project)
}

// DELETE /projects/:id
func DeleteProject(c *gin.Context) {
    id := c.Param("id")
    if err := db.DB.Delete(&models.Project{}, id).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.Status(http.StatusNoContent)
}
