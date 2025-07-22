package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/response"
	"github.com/linskybing/platform-go/services"
	"github.com/linskybing/platform-go/utils"
)

// Instantiate ConfigFile Instance godoc
// @Summary Instantiate a config file instance
// @Tags config_files
// @Security BearerAuth
// @Produce json
// @Param id path int true "Config File ID"
// @Success 200 {object} response.MessageResponse
// @Failure 400 {object} response.ErrorResponse "Invalid ID"
// @Failure 500 {object} response.ErrorResponse "Internal Server Error"
// @Router /instance/{id} [post]
func CreateInstanceHandler(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid config id"})
		return
	}
	err = services.CreateInstance(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.MessageResponse{Message: "create successfully"})
}

// Destruce ConfigFile Instance godoc
// @Summary Destruct a config file instance
// @Tags config_files
// @Security BearerAuth
// @Produce json
// @Param id path int true "Config File ID"
// @Success 204 "No content"
// @Failure 400 {object} response.ErrorResponse "Invalid ID"
// @Failure 500 {object} response.ErrorResponse "Internal Server Error"
// @Router /instance/{id} [post]
func DestructInstanceHandler(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid config id"})
		return
	}
	err = services.DeleteInstance(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
