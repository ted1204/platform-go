package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/response"
	"github.com/linskybing/platform-go/src/services"
	"github.com/linskybing/platform-go/src/utils"
)

type FormHandler struct {
	service *services.FormService
}

func NewFormHandler(service *services.FormService) *FormHandler {
	return &FormHandler{service: service}
}

func (h *FormHandler) CreateForm(c *gin.Context) {
	var input dto.CreateFormDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Unauthorized"})
		return
	}

	form, err := h.service.CreateForm(userID, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response.SuccessResponse{Data: form})
}

func (h *FormHandler) GetMyForms(c *gin.Context) {
	userID, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Unauthorized"})
		return
	}

	forms, err := h.service.GetUserForms(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{Data: forms})
}

func (h *FormHandler) GetAllForms(c *gin.Context) {
	forms, err := h.service.GetAllForms()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{Data: forms})
}

func (h *FormHandler) UpdateFormStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid ID"})
		return
	}

	var input dto.UpdateFormStatusDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	form, err := h.service.UpdateFormStatus(uint(id), input.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{Data: form})
}
