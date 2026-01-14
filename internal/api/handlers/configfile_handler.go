package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/application"
	"github.com/linskybing/platform-go/internal/domain/configfile"
	"github.com/linskybing/platform-go/pkg/response"
	"github.com/linskybing/platform-go/pkg/utils"
)

type ConfigFileHandler struct {
	svc *application.ConfigFileService
}

func NewConfigFileHandler(svc *application.ConfigFileService) *ConfigFileHandler {
	return &ConfigFileHandler{svc: svc}
}

// ListConfigFiles godoc
// @Summary List all config files
// @Tags config_files
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.ConfigFile
// @Failure 500 {object} response.ErrorResponse
// @Router /config-files [get]
func (h *ConfigFileHandler) ListConfigFilesHandler(c *gin.Context) {
	configFiles, err := h.svc.ListConfigFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, configFiles)
}

// GetConfigFile godoc
// @Summary Get a config file by ID
// @Tags config_files
// @Security BearerAuth
// @Produce json
// @Param id path int true "Config File ID"
// @Success 200 {object} models.ConfigFile
// @Failure 400 {object} response.ErrorResponse "Invalid ID"
// @Failure 404 {object} response.ErrorResponse "Not Found"
// @Router /config-files/{id} [get]
func (h *ConfigFileHandler) GetConfigFileHandler(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid config file ID"})
		return
	}

	configFile, err := h.svc.GetConfigFile(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "config file not found"})
		return
	}
	c.JSON(http.StatusOK, configFile)
}

// CreateConfigFile godoc
// @Summary Create a new config file
// @Tags config_files
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param filename formData string true "Filename"
// @Param raw_yaml formData string true "Raw YAML content"
// @Param project_id formData int true "Project ID"
// @Success 201 {object} models.ConfigFile
// @Failure 400 {object} response.ErrorResponse "Bad request"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /config-files [post]
func (h *ConfigFileHandler) CreateConfigFileHandler(c *gin.Context) {
	var input configfile.CreateConfigFileInput
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: fmt.Sprintf("Invalid input: %v", err)})
		return
	}

	if input.Filename == "" || input.RawYaml == "" || input.ProjectID == 0 {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "filename, raw_yaml, and project_id are required"})
		return
	}

	configFile, err := h.svc.CreateConfigFile(c, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, configFile)
}

// UpdateConfigFile godoc
// @Summary Update a config file by ID
// @Tags config_files
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "Config File ID"
// @Param filename formData string false "Filename"
// @Param raw_yaml formData string false "Raw YAML content"
// @Success 200 {object} models.ConfigFile
// @Failure 400 {object} response.ErrorResponse "Bad Request"
// @Failure 404 {object} response.ErrorResponse "Not Found"
// @Failure 500 {object} response.ErrorResponse "Internal Server Error"
// @Router /config-files/{id} [put]
func (h *ConfigFileHandler) UpdateConfigFileHandler(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid config file ID"})
		return
	}

	var input configfile.ConfigFileUpdateDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	updatedConfigFile, err := h.svc.UpdateConfigFile(c, uint(id), input)
	if err != nil {
		if err == application.ErrConfigFileNotFound {
			c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "config file not found"})
		} else {
			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, updatedConfigFile)
}

// DeleteConfigFile godoc
// @Summary Delete a config file
// @Tags config_files
// @Security BearerAuth
// @Param id path int true "ConfigFile ID"
// @Success 204 "No Content"
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /config-files/{id} [delete]
func (h *ConfigFileHandler) DeleteConfigFileHandler(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid config file ID"})
		return
	}

	err = h.svc.DeleteConfigFile(c, uint(id))
	if err != nil {
		if err == application.ErrConfigFileNotFound {
			c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "config file not found"})
		} else {
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// ListConfigFilesByProjectID godoc
// @Summary List config files by project ID
// @Tags config_files
// @Security BearerAuth
// @Produce json
// @Param id path int true "Project ID"
// @Success 200 {array} models.ConfigFile
// @Failure 400 {object} response.ErrorResponse "Bad Request"
// @Failure 500 {object} response.ErrorResponse "Internal Server Error"
// @Router /projects/{id}/config-files [get]
func (h *ConfigFileHandler) ListConfigFilesByProjectIDHandler(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid project_id"})
		return
	}

	configFiles, err := h.svc.ListConfigFilesByProjectID(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, configFiles)
}

// CreateInstanceHandler godoc
// @Summary Instantiate a config file instance
// @Description Creates a Kubernetes instance from a config file. Validates GPU resource requests against project MPS limits.
// GPU resources (nvidia.com/gpu) must match project MPS configuration. Non-GPU workloads skip MPS validation.
// @Tags Instance
// @Security BearerAuth
// @Produce json
// @Param id path int true "Config File ID"
// @Success 200 {object} response.MessageResponse "Instance created successfully"
// @Failure 400 {object} response.ErrorResponse "Invalid config file ID or validation error"
// @Failure 500 {object} response.ErrorResponse "Internal Server Error"
// @Router /instance/{id} [post]
func (h *ConfigFileHandler) CreateInstanceHandler(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid config id"})
		return
	}
	err = h.svc.CreateInstance(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.MessageResponse{Message: "create successfully"})
}

// Destruce ConfigFile Instance godoc
// @Summary Destruct a config file instance
// @Tags Instance
// @Security BearerAuth
// @Produce json
// @Param id path int true "Config File ID"
// @Success 204 "No content"
// @Failure 400 {object} response.ErrorResponse "Invalid ID"
// @Failure 500 {object} response.ErrorResponse "Internal Server Error"
// @Router /instance/{id} [delete]
func (h *ConfigFileHandler) DestructInstanceHandler(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid config id"})
		return
	}
	err = h.svc.DeleteInstance(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
