package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/application"
	"github.com/linskybing/platform-go/pkg/response"
	"github.com/linskybing/platform-go/pkg/utils"
)

type ImageHandler struct {
	service *application.ImageService
}

func NewImageHandler(service *application.ImageService) *ImageHandler {
	return &ImageHandler{service: service}
}

// @Summary Submit image request
// @Description User submits a request to use a specific container image
// @Tags Images
// @Accept json
// @Produce json
// @Param request body image.CreateImageRequestDTO true "Image Request Payload"
// @Success 201 {object} response.SuccessResponse{data=image.ImageRequest}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /images/requests [post]
func (h *ImageHandler) SubmitRequest(c *gin.Context) {
	var payload struct {
		Registry  string `json:"registry"`
		Name      string `json:"name" binding:"required"`
		Tag       string `json:"tag" binding:"required"`
		ProjectID *uint  `json:"project_id"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}
	uid, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "unauthorized"})
		return
	}

	req, err := h.service.SubmitRequest(uid, payload.Registry, payload.Name, payload.Tag, payload.ProjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, response.SuccessResponse{Data: req})
}

// @Summary List image requests
// @Description List all image requests with optional filtering
// @Tags Images
// @Accept json
// @Produce json
// @Param status query string false "Filter by status (pending, approved, rejected)"
// @Param project_id query int false "Filter by Project ID"
// @Success 200 {object} response.SuccessResponse{data=[]image.ImageRequest}
// @Failure 500 {object} response.ErrorResponse
// @Router /images/requests [get]
func (h *ImageHandler) ListRequests(c *gin.Context) {
	status := c.Query("status")
	var projectID *uint

	if pIDStr := c.Query("project_id"); pIDStr != "" {
		if id, err := strconv.ParseUint(pIDStr, 10, 64); err == nil {
			pid := uint(id)
			projectID = &pid
		}
	}

	reqs, err := h.service.ListRequests(projectID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.SuccessResponse{Data: reqs})
}

// @Summary Approve image request
// @Description Admin approves an image request
// @Tags Images
// @Accept json
// @Produce json
// @Param id path int true "Request ID"
// @Param request body image.UpdateImageRequestDTO true "Approval Note"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /images/requests/{id}/approve [post]
func (h *ImageHandler) ApproveRequest(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid id"})
		return
	}
	var payload struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&payload)

	approverID, uidErr := utils.GetUserIDFromContext(c)
	if uidErr != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "unauthorized"})
		return
	}

	if err := h.service.ApproveRequest(uint(id), payload.Note, approverID); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.SuccessResponse{Message: "Request approved"})
}

// @Summary Reject image request
// @Description Admin rejects an image request
// @Tags Images
// @Accept json
// @Produce json
// @Param id path int true "Request ID"
// @Param request body image.UpdateImageRequestDTO true "Rejection Note"
// @Success 200 {object} response.SuccessResponse{data=image.ImageRequest}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /images/requests/{id}/reject [post]
func (h *ImageHandler) RejectRequest(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid id"})
		return
	}
	var payload struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&payload)

	approverID, uidErr := utils.GetUserIDFromContext(c)
	if uidErr != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "unauthorized"})
		return
	}

	req, err := h.service.RejectRequest(uint(id), payload.Note, approverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.SuccessResponse{Data: req})
}

// @Summary List allowed images
// @Description List allowed images for a project or globally
// @Tags Images
// @Accept json
// @Produce json
// @Param id path int false "Project ID (optional path param)"
// @Param project_id query int false "Project ID (optional query param)"
// @Success 200 {object} response.SuccessResponse{data=[]image.AllowedImageDTO}
// @Failure 500 {object} response.ErrorResponse
// @Router /images/allowed [get]
func (h *ImageHandler) ListAllowed(c *gin.Context) {
	projectIDStr := c.Param("id")
	if projectIDStr == "" {
		projectIDStr = c.Query("project_id")
	}

	var projectID *uint
	if projectIDStr != "" {
		if id, err := strconv.ParseUint(projectIDStr, 10, 64); err == nil {
			pid := uint(id)
			projectID = &pid
		}
	}

	imgs, err := h.service.ListAllowedImages(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.SuccessResponse{Data: imgs})
}

// @Summary Add image to project
// @Description Project Manager adds a new image directly to a project
// @Tags Projects
// @Accept json
// @Produce json
// @Param id path int true "Project ID"
// @Param request body image.CreateProjectImageDTO true "Image Details"
// @Success 201 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{id}/images [post]
func (h *ImageHandler) AddProjectImage(c *gin.Context) {
	projectIDStr := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid project_id"})
		return
	}

	var payload struct {
		Name string `json:"name" binding:"required"`
		Tag  string `json:"tag" binding:"required"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	uid, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "unauthorized"})
		return
	}

	err = h.service.AddProjectImage(uid, uint(projectID), payload.Name, payload.Tag)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, response.SuccessResponse{
		Message: "Image request created for project",
	})
}

// @Summary Remove image from project
// @Description Disable an allowed image rule for a project
// @Tags Projects
// @Accept json
// @Produce json
// @Param id path int true "Project ID"
// @Param rule_id path int true "Allow List Rule ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /projects/{id}/images/{rule_id} [delete]
func (h *ImageHandler) RemoveProjectImage(c *gin.Context) {
	ruleID, err := utils.ParseIDParam(c, "rule_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid rule id"})
		return
	}

	if err := h.service.DisableAllowListRule(uint(ruleID)); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.SuccessResponse{Message: "image rule disabled"})
}

// @Summary Pull images
// @Description Trigger async job to pull images to the cluster
// @Tags Images
// @Accept json
// @Produce json
// @Param request body object{names=[]string} true "List of images to pull (e.g. ['nginx:latest'])"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /images/pull [post]
func (h *ImageHandler) PullImage(c *gin.Context) {
	var payload struct {
		Names []string `json:"names" binding:"required"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	type PullRequest struct {
		Name string
		Tag  string
	}

	var requests []PullRequest
	for _, fullImage := range payload.Names {
		var name, tag string

		lastColon := strings.LastIndex(fullImage, ":")
		if lastColon > 0 {
			name = fullImage[:lastColon]
			tag = fullImage[lastColon+1:]
		} else {
			name = fullImage
			tag = "latest"
		}

		requests = append(requests, PullRequest{Name: name, Tag: tag})
	}

	var jobIDs []string
	for _, req := range requests {
		jobID, err := h.service.PullImageAsync(req.Name, req.Tag)
		if err != nil {
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
			return
		}
		jobIDs = append(jobIDs, jobID)
	}

	c.JSON(http.StatusOK, response.SuccessResponse{Data: gin.H{
		"job_ids": jobIDs,
		"message": "Images queued for pulling",
	}})
}

// @Summary Get pull job status
// @Description Get the status of an image pull job
// @Tags Images
// @Accept json
// @Produce json
// @Param job_id path string true "Job ID"
// @Success 200 {object} response.SuccessResponse{data=application.PullJobStatus}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /images/pull/{job_id} [get]
func (h *ImageHandler) GetPullJobStatus(c *gin.Context) {
	jobID := c.Param("job_id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "job_id required"})
		return
	}

	status := h.service.GetPullJobStatus(jobID)
	if status == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "job not found"})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{Data: status})
}

// @Summary List failed pull jobs
// @Description Get a list of recently failed image pull jobs
// @Tags Images
// @Accept json
// @Produce json
// @Param limit query int false "Limit number of results (default 10)"
// @Success 200 {object} response.SuccessResponse{data=[]application.PullJobStatus}
// @Router /images/pull/failed [get]
func (h *ImageHandler) GetFailedPullJobs(c *gin.Context) {
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	failedJobs := h.service.GetFailedPullJobs(limit)
	c.JSON(http.StatusOK, response.SuccessResponse{Data: failedJobs})
}

// @Summary List active pull jobs
// @Description Get a list of currently active image pull jobs
// @Tags Images
// @Accept json
// @Produce json
// @Success 200 {object} response.SuccessResponse{data=[]application.PullJobStatus}
// @Router /images/pull/active [get]
func (h *ImageHandler) GetActivePullJobs(c *gin.Context) {
	activeJobs := h.service.GetActivePullJobs()
	c.JSON(http.StatusOK, response.SuccessResponse{Data: activeJobs})
}

// @Summary Delete allowed image rule
// @Description Disable a global allowed image rule
// @Tags Images
// @Accept json
// @Produce json
// @Param id path int true "Allow List Rule ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /images/allowed/{id} [delete]
func (h *ImageHandler) DeleteAllowedImage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.service.DisableAllowListRule(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.SuccessResponse{Message: "image rule disabled"})
}
