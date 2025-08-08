package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/response"
	"github.com/linskybing/platform-go/services"
	"github.com/linskybing/platform-go/utils"
)

// ListConfigFiles godoc
// @Summary List all config files
// @Tags config_files
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.ConfigFile
// @Failure 500 {object} response.ErrorResponse
// @Router /config-files [get]
func ListConfigFilesHandler(c *gin.Context) {
	configFiles, err := services.ListConfigFiles()
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
func GetConfigFileHandler(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid config file ID"})
		return
	}

	configFile, err := services.GetConfigFile(uint(id))
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
func CreateConfigFileHandler(c *gin.Context) {
	var input dto.CreateConfigFileInput
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}
	configFile, err := services.CreateConfigFile(c, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
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
func UpdateConfigFileHandler(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid config file ID"})
		return
	}

	var input dto.ConfigFileUpdateDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	updatedConfigFile, err := services.UpdateConfigFile(c, uint(id), input)
	if err != nil {
		if err == services.ErrConfigFileNotFound {
			c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "config file not found"})
		} else {
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
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
func DeleteConfigFileHandler(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid config file ID"})
		return
	}

	err = services.DeleteConfigFile(c, uint(id))
	if err != nil {
		if err == services.ErrConfigFileNotFound {
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
func ListConfigFilesByProjectIDHandler(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid project_id"})
		return
	}

	configFiles, err := services.ListConfigFilesByProjectID(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, configFiles)
}
