package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/models"
	"github.com/linskybing/platform-go/response"
	"github.com/linskybing/platform-go/utils"
)

// GetProjects godoc
// @Summary      Get all projects
// @Description  Returns a list of all projects
// @Tags         projects
// @Produce      json
// @Success      200  {array}  models.Project
// @Failure      500  {object}  response.ErrorResponse
// @Router       /projects [get]
func GetProjects(c *gin.Context) {
	var projects []models.Project
	if err := db.DB.Find(&projects).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, projects)
}

// GetProjectByID godoc
// @Summary      Get a project by ID
// @Description  Returns a single project by its ID
// @Tags         projects
// @Produce      json
// @Param        id   path      int  true  "Project ID"
// @Success      200  {object}  models.Project
// @Failure      404  {object}  response.ErrorResponse
// @Router       /projects/{id} [get]
func GetProjectByID(c *gin.Context) {
	id := c.Param("id")
	var project models.Project

	if err := db.DB.First(&project, id).Error; err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "Project not found"})
		return
	}
	c.JSON(http.StatusOK, project)
}

// CreateProject godoc
// @Summary      Create a new project
// @Description  Creates a project with name, group and optional description
// @Tags         projects
// @Accept multipart/form-data
// @Produce      json
// @Param        project  body      dto.CreateProjectDTO  true  "Project to create"
// @Success      201      {object}  models.Project
// @Failure      400      {object}  response.ErrorResponse
// @Failure      500      {object}  response.ErrorResponse
// @Router       /projects [post]
func CreateProject(c *gin.Context) {
	var input dto.CreateProjectDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	project := models.Project{
		ProjectName: input.ProjectName,
		GID:         input.GID,
	}
	if input.Description != nil {
		project.Description = *input.Description
	}

	if err := db.DB.Create(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	// audit log - create
	userid, _ := utils.GetUserIDFromContext(c)
	if err := utils.LogAudit(c, userid, "create", "project", project.PID, nil, project, nil); err != nil {
		log.Printf("Audit log failed: %v", err)
	}

	c.JSON(http.StatusCreated, project)
}

// UpdateProject godoc
// @Summary      Update a project
// @Description  Update a project by ID. All fields are optional; only provided fields will be updated.
// @Tags         projects
// @Accept       multipart/form-data
// @Produce      json
// @Param        id            path      int    true   "Project ID"
// @Param        project_name  formData  string false  "Project name"
// @Param        gid           formData  uint   false  "Group ID"
// @Param        description   formData  string false  "Project description"
// @Success      200           {object}  models.Project
// @Failure      400           {object}  response.ErrorResponse
// @Failure      403           {object}  response.ErrorResponse
// @Failure      404           {object}  response.ErrorResponse
// @Failure      500           {object}  response.ErrorResponse
// @Router       /projects/{id} [put]
func UpdateProject(c *gin.Context) {
	id := c.Param("id")
	var project models.Project
	if err := db.DB.First(&project, id).Error; err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "Project not found"})
		return
	}

	var input dto.UpdateProjectDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	userID, _ := utils.GetUserIDFromContext(c)
	gids := []uint{project.GID}
	if input.GID != nil && project.GID != *input.GID {
		gids = append(gids, *input.GID)
	}

	ok, err := utils.CheckProjectCreatePermission(userID, gids)
	if err != nil {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Error: err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "permission denied"})
		return
	}

	// deep copy for audit
	oldValue := project

	if input.ProjectName != nil {
		project.ProjectName = *input.ProjectName
	}
	if input.GID != nil {
		project.GID = *input.GID
	}
	if input.Description != nil {
		project.Description = *input.Description
	}

	if err := db.DB.Save(&project).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	// audit log
	if err := utils.LogAudit(c, userID, "update", "project", project.PID, oldValue, project, nil); err != nil {
		log.Printf("Audit log failed: %v", err)
	}

	c.JSON(http.StatusOK, project)
}

// DeleteProject godoc
// @Summary      Delete a project by ID
// @Description  Deletes a project by its ID
// @Tags         projects
// @Produce      json
// @Param        id path int true "Project ID"
// @Success      204 {string} string "No Content"
// @Failure      403 {object} response.ErrorResponse
// @Failure      404 {object} response.ErrorResponse
// @Failure      500 {object} response.ErrorResponse
// @Router       /projects/{id} [delete]
func DeleteProject(c *gin.Context) {
	userID, _ := utils.GetUserIDFromContext(c)
	id := c.Param("id")

	// get old record for audit
	var oldProject models.Project
	if err := db.DB.First(&oldProject, id).Error; err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "Project not found"})
		return
	}

	// permission check
	gids := []uint{oldProject.GID}
	ok, err := utils.CheckProjectCreatePermission(userID, gids)
	if err != nil {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Error: err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "permission denied"})
		return
	}

	if err := db.DB.Delete(&models.Project{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	// audit log - delete
	if err := utils.LogAudit(c, userID, "delete", "project", oldProject.PID, oldProject, nil, nil); err != nil {
		log.Printf("Audit log failed: %v", err)
	}

	c.Status(http.StatusNoContent)
}
