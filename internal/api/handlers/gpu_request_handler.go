package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/application"
	"github.com/linskybing/platform-go/internal/domain/gpu"
	"github.com/linskybing/platform-go/pkg/response"
	"github.com/linskybing/platform-go/pkg/types"
	"github.com/linskybing/platform-go/pkg/utils"
)

type GPURequestHandler struct {
	svc        *application.GPURequestService
	projectSvc *application.ProjectService
}

func NewGPURequestHandler(svc *application.GPURequestService, projectSvc *application.ProjectService) *GPURequestHandler {
	return &GPURequestHandler{svc: svc, projectSvc: projectSvc}
}

// CreateRequest godoc
// @Summary Create a GPU request
// @Tags gpu-requests
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path uint true "Project ID"
// @Param request body dto.CreateGPURequestDTO true "Request body"
// @Success 201 {object} models.GPURequest
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{id}/gpu-requests [post]
func (h *GPURequestHandler) CreateRequest(c *gin.Context) {
	projectID, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid project id"})
		return
	}

	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "unauthorized"})
		return
	}

	// Check permission: User must be project member (group member)
	project, err := h.projectSvc.GetProject(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "project not found"})
		return
	}

	// Check if user is member of the group the project belongs to
	hasPermission, err := utils.CheckGroupPermission(userID, project.GID, h.svc.Repos.View)
	if err != nil || !hasPermission {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "permission denied"})
		return
	}

	var input gpu.CreateGPURequestDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	req, err := h.svc.CreateRequest(projectID, userID, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, req)
}

// ListRequestsByProject godoc
// @Summary List GPU requests for a project
// @Tags gpu-requests
// @Security BearerAuth
// @Produce json
// @Param id path uint true "Project ID"
// @Success 200 {array} models.GPURequest
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{id}/gpu-requests [get]
func (h *GPURequestHandler) ListRequestsByProject(c *gin.Context) {
	projectID, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid project id"})
		return
	}

	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "unauthorized"})
		return
	}

	// Check permission: User must be member of the group
	project, err := h.projectSvc.GetProject(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "project not found"})
		return
	}

	// Check if user is member of the group (any role including regular user)
	hasPermission, err := utils.CheckGroupPermission(userID, project.GID, h.svc.Repos.View)
	if err != nil || !hasPermission {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "permission denied"})
		return
	}

	reqs, err := h.svc.ListByProject(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, reqs)
}

// ListPendingRequests godoc
// @Summary List all pending GPU requests (Admin only)
// @Tags gpu-requests
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.GPURequest
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /admin/gpu-requests [get]
func (h *GPURequestHandler) ListPendingRequests(c *gin.Context) {
	claimsVal, _ := c.Get("claims")
	claims := claimsVal.(*types.Claims)
	if !claims.IsAdmin {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "permission denied"})
		return
	}

	reqs, err := h.svc.ListPending()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, reqs)
}

// ProcessRequest godoc
// @Summary Approve or Reject a GPU request (Admin only)
// @Tags gpu-requests
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path uint true "Request ID"
// @Param body body dto.UpdateGPURequestStatusDTO true "Status"
// @Success 200 {object} models.GPURequest
// @Failure 400 {object} response.ErrorResponse
// @Failure 403 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /admin/gpu-requests/{id}/status [put]
func (h *GPURequestHandler) ProcessRequest(c *gin.Context) {
	claimsVal, _ := c.Get("claims")
	claims := claimsVal.(*types.Claims)
	if !claims.IsAdmin {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "permission denied"})
		return
	}

	requestID, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid request id"})
		return
	}

	var input gpu.UpdateGPURequestStatusDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	req, err := h.svc.ProcessRequest(requestID, input.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, req)
}
