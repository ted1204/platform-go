package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/application"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/pkg/response"
	"github.com/linskybing/platform-go/pkg/types"
	"github.com/linskybing/platform-go/pkg/utils"
)

type ProjectHandler struct {
	svc *application.ProjectService
}

func NewProjectHandler(svc *application.ProjectService) *ProjectHandler {
	return &ProjectHandler{svc: svc}
}

// GetProjects godoc
// @Summary List projects for current user
// @Tags projects
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.Project
// @Failure 500 {object} response.ErrorResponse
// @Router /projects [get]
func (h *ProjectHandler) GetProjects(c *gin.Context) {
	// Get project views for this user
	projectViews, err := h.svc.ListProjects()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	// Return empty array if no projects
	if len(projectViews) == 0 {
		c.JSON(http.StatusOK, []project.Project{})
		return
	}

	// Get full project details for each project ID
	result := make([]project.Project, 0, len(projectViews))
	seenPIDs := make(map[uint]bool)

	for _, pv := range projectViews {
		// Avoid duplicates (user might be in multiple groups for same project)
		if seenPIDs[pv.PID] {
			continue
		}
		seenPIDs[pv.PID] = true

		// Get full project details from repository
		fullProject, err := h.svc.GetProject(pv.PID)
		if err == nil {
			result = append(result, *fullProject)
		}
	}

	c.JSON(http.StatusOK, result)
}

// GetProjectsByUser godoc
// @Summary List projects by user
// @Tags projects
// @Security BearerAuth
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {array} map[string]project.GroupProjects
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/by-user [get]
func (h *ProjectHandler) GetProjectsByUser(c *gin.Context) {
	id, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid id"})
		return
	}
	records, err := h.svc.GetProjectsByUser(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	grouped := h.svc.GroupProjectsByGID(records)

	// Return empty object if no projects
	if grouped == nil {
		grouped = make(map[string]map[string]interface{})
	}

	c.JSON(http.StatusOK, grouped)
}

// GetProjectByID godoc
// @Summary Get project by ID
// @Tags projects
// @Security BearerAuth
// @Produce json
// @Param id path uint true "Project ID"
// @Success 200 {object} models.Project
// @Failure 400 {object} response.ErrorResponse "Invalid project id"
// @Failure 404 {object} response.ErrorResponse "Project not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /projects/{id} [get]
func (h *ProjectHandler) GetProjectByID(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid project id"})
		return
	}
	project, err := h.svc.GetProject(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "project not found"})
		return
	}
	c.JSON(http.StatusOK, project)
}

// CreateProject godoc
// @Summary Create a new project
// @Tags projects
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param project_name formData string true "Project name"
// @Param description formData string false "Description"
// @Param g_id formData uint true "Group ID"
// @Success 201 {object} models.Project
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /projects [post]
func (h *ProjectHandler) CreateProject(c *gin.Context) {
	var input project.CreateProjectDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	// Only super admin can set GPU quota and access
	claimsVal, _ := c.Get("claims")
	claims := claimsVal.(*types.Claims)
	if !claims.IsAdmin {
		input.GPUQuota = nil
		input.GPUAccess = nil
	}

	project, err := h.svc.CreateProject(c, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, project)
}

// UpdateProject godoc
// @Summary Update project by ID
// @Tags projects
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param id path uint true "Project ID"
// @Param project_name formData string false "Project name"
// @Param description formData string false "Description"
// @Param g_id formData uint false "Group ID"
// @Success 200 {object} models.Project
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 404 {object} response.ErrorResponse "Project not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /projects/{id} [put]
func (h *ProjectHandler) UpdateProject(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid project id"})
		return
	}
	var input project.UpdateProjectDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	// Only super admin can set GPU quota and access
	claimsVal, _ := c.Get("claims")
	claims := claimsVal.(*types.Claims)
	if !claims.IsAdmin {
		input.GPUQuota = nil
		input.GPUAccess = nil
	}

	project, err := h.svc.UpdateProject(c, id, input)
	if err != nil {
		if errors.Is(err, application.ErrProjectNotFound) {
			c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "project not found"})
		} else {
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, project)
}

// DeleteProject godoc
// @Summary Delete project by ID
// @Tags projects
// @Security BearerAuth
// @Produce json
// @Param id path uint true "Project ID"
// @Success 200 {object} response.MessageResponse "Project deleted"
// @Failure 400 {object} response.ErrorResponse "Invalid project id"
// @Failure 404 {object} response.ErrorResponse "Project not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /projects/{id} [delete]
func (h *ProjectHandler) DeleteProject(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid project id"})
		return
	}

	err = h.svc.DeleteProject(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.MessageResponse{Message: "project deleted"})
}
