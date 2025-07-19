package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/response"
	"github.com/linskybing/platform-go/services"
	"github.com/linskybing/platform-go/utils"
)

// ListResourcesByProjectID godoc
// @Summary List resources by project ID
// @Tags resources
// @Security BearerAuth
// @Produce json
// @Param id path uint true "Project ID"
// @Success 200 {array} models.ResourceSwagger
// @Failure 400 {object} response.ErrorResponse "Invalid project ID"
// @Router /projects/{id}/resources [get]
func ListResourcesByProjectID(c *gin.Context) {
	projectID, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid project id"})
		return
	}

	resources, err := services.ListResourcesByProjectID(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, resources)
}

// ListResourcesByConfigFileID godoc
// @Summary List resources by config file ID
// @Tags resources
// @Security BearerAuth
// @Produce json
// @Param id path uint true "Config File ID"
// @Success 200 {array} models.ResourceSwagger
// @Failure 400 {object} response.ErrorResponse "Invalid config file ID"
// @Router /config-files/{id}/resources [get]
func ListResourcesByConfigFileID(c *gin.Context) {
	cfID, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid config file id"})
		return
	}

	resources, err := services.ListResourcesByConfigFileID(cfID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, resources)
}

// GetResource godoc
// @Summary Get resource by ID
// @Tags resources
// @Security BearerAuth
// @Produce json
// @Param id path uint true "Resource ID"
// @Success 200 {object} models.ResourceSwagger
// @Failure 400 {object} response.ErrorResponse "Invalid ID"
// @Failure 404 {object} response.ErrorResponse "Resource not found"
// @Router /resources/{id} [get]
func GetResource(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid resource id"})
		return
	}

	resource, err := services.GetResource(id)
	if err != nil || resource == nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "resource not found"})
		return
	}

	c.JSON(http.StatusOK, resource)
}

// UpdateResource godoc
// @Summary Update resource by ID
// @Tags resources
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param id path uint true "Resource ID"
// @Param name formData string false "Resource name"
// @Param type formData string false "Resource type (e.g. pod, service, deployment)"
// @Param parsed_yaml formData string false "Parsed YAML content"
// @Param description formData string false "Description"
// @Success 200 {object} models.ResourceSwagger
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 404 {object} response.ErrorResponse "Resource not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /resources/{id} [put]
func UpdateResource(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid resource id"})
		return
	}

	var input dto.ResourceUpdateDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	resource, err := services.UpdateResource(c, id, input)
	if err != nil {
		if errors.Is(err, services.ErrResourceNotFound) {
			c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "resource not found"})
		} else {
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, resource)
}

// DeleteResource godoc
// @Summary Delete resource by ID
// @Tags resources
// @Security BearerAuth
// @Produce json
// @Param id path uint true "Resource ID"
// @Success 204 "No Content"
// @Failure 400 {object} response.ErrorResponse "Invalid ID"
// @Failure 404 {object} response.ErrorResponse "Resource not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /resources/{id} [delete]
func DeleteResource(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid resource id"})
		return
	}

	err = services.DeleteResource(c, id)
	if err != nil {
		if errors.Is(err, services.ErrResourceNotFound) {
			c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "resource not found"})
		} else {
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		}
		return
	}

	c.Status(http.StatusNoContent)
}
