package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/response"
	"github.com/linskybing/platform-go/services"
	"github.com/linskybing/platform-go/utils"
)

// GetUsers godoc
// @Summary List all users
// @Tags users
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.UserWithSuperAdmin
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /users [get]
func GetUsers(c *gin.Context) {
	users, err := services.ListUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

// ListUsersPaging godoc
// @Summary List all users with pagination
// @Tags users
// @Security BearerAuth
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 10, max: 100)"
// @Success 200 {object} response.SuccessResponse{data=[]models.UserWithSuperAdmin}
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /users/paging [get]
func ListUsersPaging(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	users, err := services.ListUserByPaging(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.SuccessResponse{Data: users})
}

// GetUserByID godoc
// @Summary Get user by ID
// @Tags users
// @Security BearerAuth
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} models.UserWithSuperAdmin
// @Failure 400 {object} response.ErrorResponse "Invalid user id"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /users/{id} [get]
func GetUserByID(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid user id"})
		return
	}

	user, err := services.FindUserByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "User not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// UpdateUser updates the information of a user by ID.
// @Summary Update user
// @Security BearerAuth
// @Description Partially update user's email, full name, type, status, or password.
// @Tags users
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "User ID"
// @Param old_password formData string false "Old password (required if updating password)"
// @Param password formData string false "New password"
// @Param email formData string false "Email"
// @Param full_name formData string false "Full name"
// @Param type formData string false "User type: origin or oauth2"
// @Param status formData string false "User status: online, offline, delete"
// @Success 200 {object} dto.UserDTO "Updated user info"
// @Failure 400 {object} response.ErrorResponse "Bad request error"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /users/{id} [put]
func UpdateUser(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid user id"})
		return
	}

	var input dto.UpdateUserInput
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	updatedUser, err := services.UpdateUser(id, input)
	if err != nil {
		switch err {
		case services.ErrUserNotFound:
			c.JSON(http.StatusNotFound, response.ErrorResponse{Error: err.Error()})
		case services.ErrMissingOldPassword, services.ErrIncorrectPassword:
			c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, updatedUser)
}

// DeleteUser godoc
// @Summary Delete user by ID
// @Tags users
// @Security BearerAuth
// @Produce json
// @Param id path int true "User ID"
// @Success 204 "No Content"
// @Failure 400 {object} response.ErrorResponse "Invalid user id"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /users/{id} [delete]
func DeleteUser(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid user id"})
		return
	}

	if err := services.RemoveUser(id); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
