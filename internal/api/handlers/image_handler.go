package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/application"
	"github.com/linskybing/platform-go/internal/domain/image"
	"github.com/linskybing/platform-go/pkg/response"
	"github.com/linskybing/platform-go/pkg/utils"
)

type ImageHandler struct {
	service *application.ImageService
}

func NewImageHandler(service *application.ImageService) *ImageHandler {
	return &ImageHandler{service: service}
}

// Submit an image request (user)
func (h *ImageHandler) SubmitRequest(c *gin.Context) {
	var payload struct {
		Name      string `json:"name" binding:"required"`
		Tag       string `json:"tag" binding:"required"`
		ProjectID *uint  `json:"project_id"` // nil for global request
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
	req, err := h.service.SubmitRequest(uid, payload.Name, payload.Tag, payload.ProjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, response.SuccessResponse{Data: req})
}

// List image requests (admin)
func (h *ImageHandler) ListRequests(c *gin.Context) {
	status := c.Query("status")
	reqs, err := h.service.ListRequests(status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.SuccessResponse{Data: reqs})
}

// Approve an image request
func (h *ImageHandler) ApproveRequest(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid id"})
		return
	}
	var payload struct {
		Note     string `json:"note"`
		IsGlobal bool   `json:"is_global"` // admin can choose to make it global
	}
	_ = c.ShouldBindJSON(&payload)
	req, err := h.service.ApproveRequest(uint(id), payload.Note, payload.IsGlobal)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.SuccessResponse{Data: req})
}

// Reject an image request
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
	req, err := h.service.RejectRequest(uint(id), payload.Note)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.SuccessResponse{Data: req})
}

// List allowed images for dropdowns
func (h *ImageHandler) ListAllowed(c *gin.Context) {
	// Check if project_id is provided in URL path or query
	projectIDStr := c.Param("id") // from /projects/:id/images
	if projectIDStr == "" {
		projectIDStr = c.Query("project_id") // fallback to query parameter
	}

	var imgs []image.AllowedImage
	var err error

	if projectIDStr != "" {
		projectID, parseErr := strconv.ParseUint(projectIDStr, 10, 64)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid project_id"})
			return
		}
		// Return global + project-specific images
		imgs, err = h.service.ListAllowedForProject(uint(projectID))
	} else {
		// Return all images (for admin)
		imgs, err = h.service.ListAllowed()
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.SuccessResponse{Data: imgs})
}

// AddProjectImage allows project managers to add images to their project
func (h *ImageHandler) AddProjectImage(c *gin.Context) {
	projectIDStr := c.Param("id") // from /projects/:id/images
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

	img, err := h.service.AddProjectImage(uid, uint(projectID), payload.Name, payload.Tag)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, response.SuccessResponse{
		Code:    0,
		Message: "Image added to project",
		Data:    img,
	})
}

// Trigger cluster pull (admin)
func (h *ImageHandler) PullImage(c *gin.Context) {
	var payload struct {
		Name string `json:"name" binding:"required"`
		Tag  string `json:"tag" binding:"required"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}
	// TODO: implement actual pull. For now, accept and return success.
	if err := h.service.PullImage(payload.Name, payload.Tag); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.SuccessResponse{Message: "pull queued"})
}

// Delete allowed image (admin)
func (h *ImageHandler) DeleteAllowedImage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.service.DeleteAllowedImage(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.SuccessResponse{Message: "image deleted"})
}
