package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/models"
	"github.com/linskybing/platform-go/src/response"
	"github.com/linskybing/platform-go/src/services"
	"github.com/linskybing/platform-go/src/types"
	"github.com/linskybing/platform-go/src/utils"
)

type GPURequestHandler struct {
	svc        *services.GPURequestService
	projectSvc *services.ProjectService
}

func NewGPURequestHandler(svc *services.GPURequestService, projectSvc *services.ProjectService) *GPURequestHandler {
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

	// Check permission: User must be project admin (group admin)
	project, err := h.projectSvc.GetProject(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "project not found"})
		return
	}

	// Check if user is admin of the group the project belongs to
	hasPermission, err := utils.CheckGroupAdminPermission(userID, project.GID, h.svc.Repos.View)
	if err != nil || !hasPermission {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "permission denied"})
		return
	}

	var input dto.CreateGPURequestDTO
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

	// Check permission: User must be member of the group or super admin
	project, err := h.projectSvc.GetProject(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "project not found"})
		return
	}

	// Check if user is in the group (any role)
	// We can use CheckGroupPermission which checks for manager, but we want any member.
	// Or just check if user is in group.
	// Let's assume CheckGroupPermission is enough for now, or we can implement a simpler check.
	// Actually, let's just allow if they are in the group.
	// For now, let's use CheckGroupPermission (which checks for manager or super admin).
	// Wait, normal users should be able to see requests? Maybe not.
	// Requirement: "Project Admin cannot modify only submit request".
	// "User can submit request electronic form" -> Wait, "User can submit request"?
	// The prompt says: "Project Admin cannot modify only submit request form. Also user can submit request electronic form".
	// This implies users can also submit requests? Or maybe "User" here refers to the Project Admin as a user of the system.
	// "Project Admin cannot modify only submit request form" AND "Also user can submit request electronic form".
	// This might mean ANY user in the project can submit a request?
	// Let's assume Project Admin for now as per "Project Admin cannot modify only submit request".
	// If "User" means any user, then I should relax the permission.
	// Let's stick to Project Admin (Group Admin) for submission.
	// For listing, let's allow Project Admin.

	hasPermission, err := utils.CheckGroupAdminPermission(userID, project.GID, h.svc.Repos.View)
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

	var input dto.UpdateGPURequestStatusDTO
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
