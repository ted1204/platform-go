package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/linskybing/platform-go/internal/application"
	"github.com/linskybing/platform-go/internal/config"
	"github.com/linskybing/platform-go/internal/domain/user"
	"github.com/linskybing/platform-go/pkg/response"
	"github.com/linskybing/platform-go/pkg/utils"
	"gorm.io/gorm"
)

type UserHandler struct {
	svc *application.UserService
}

func NewUserHandler(svc *application.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// Register godoc
// @Summary User registration
// @Tags auth
// @Accept x-www-form-urlencoded
// @Produce json
// @Param input body user.CreateUserInput true "User registration info"
// @Success 201 {object} response.MessageResponse "User registered successfully"
// @Failure 400 {object} response.ErrorResponse "Invalid input"
// @Failure 409 {object} response.ErrorResponse "Username already taken"
// @Failure 500 {object} response.ErrorResponse "Failed to create user"
// @Router /register [post]
func (h *UserHandler) Register(c *gin.Context) {
	var input user.CreateUserInput

	if err := c.ShouldBind(&input); err != nil {
		// Try to produce friendly validation messages for the frontend
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			msgs := make([]string, 0, len(verr))

			labels := map[string]string{
				"Username": "username",
				"Password": "password",
				"Email":    "email",
				"FullName": "full name",
				"Type":     "type",
				"Status":   "status",
			}

			for _, fe := range verr {
				field := fe.StructField()
				lbl, ok := labels[field]
				if !ok {
					lbl = strings.ToLower(field)
				}

				var msg string
				switch fe.Tag() {
				case "required":
					msg = fmt.Sprintf("%s is required", lbl)
				case "min":
					msg = fmt.Sprintf("%s must be at least %s characters", lbl, fe.Param())
				case "max":
					msg = fmt.Sprintf("%s must be at most %s characters", lbl, fe.Param())
				case "email":
					msg = fmt.Sprintf("%s must be a valid email address", lbl)
				case "oneof":
					msg = fmt.Sprintf("%s must be one of [%s]", lbl, fe.Param())
				default:
					msg = fmt.Sprintf("%s is invalid", lbl)
				}
				msgs = append(msgs, msg)
			}

			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: strings.Join(msgs, "; ")})
			return
		}

		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid input"})
		return
	}

	err := h.svc.RegisterUser(input)
	if err != nil {
		if errors.Is(err, application.ErrUsernameTaken) {
			c.JSON(http.StatusConflict, response.ErrorResponse{Error: err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, response.MessageResponse{Message: "User registered successfully"})
}

// Login godoc
// @Summary User login
// @Tags auth
// @Accept x-www-form-urlencoded
// @Produce json
// @Param username formData string true "Username"
// @Param password formData string true "Password"
// @Success 200 {object} response.TokenResponse "JWT token and user info"
// @Failure 400 {object} response.ErrorResponse "Invalid input"
// @Failure 401 {object} response.ErrorResponse "Invalid username or password"
// @Failure 500 {object} response.ErrorResponse "Failed to generate token"
// @Router /login [post]
func (h *UserHandler) Login(c *gin.Context) {
	var req struct {
		Username string `form:"username" binding:"required"`
		Password string `form:"password" binding:"required"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid input"})
		return
	}

	user, token, isAdmin, err := h.svc.LoginUser(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Invalid username or password"})
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		"token",
		token,
		3600,
		"/",
		"",
		config.IsProduction, // Secure only in production
		true,
	)

	c.JSON(http.StatusOK, response.TokenResponse{
		Token:    token,
		UID:      user.UID,
		Username: user.Username,
		IsAdmin:  isAdmin,
	})
}

// Logout godoc
// @Summary User logout
// @Tags auth
// @Produce json
// @Success 200 {object} response.BasicResponse "Logout successful"
// @Router /logout [post]
func (h *UserHandler) Logout(c *gin.Context) {
	c.SetCookie(
		"token",
		"",
		-1,
		"/",
		"",
		false,
		true,
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "Logout successful",
	})
}

// ForgotPassword godoc
// @Summary Reset password by username (no verification required)
// @Tags auth
// @Accept x-www-form-urlencoded
// @Produce json
// @Param username formData string true "Username"
// @Param new_password formData string true "New password (minimum 6 characters)"
// @Success 200 {object} response.MessageResponse "Password reset successfully"
// @Failure 400 {object} response.ErrorResponse "Invalid input"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 500 {object} response.ErrorResponse "Failed to reset password"
// @Router /forgot-password [post]
func (h *UserHandler) ForgotPassword(c *gin.Context) {
	var input user.ForgotPasswordInput

	if err := c.ShouldBind(&input); err != nil {
		var verr validator.ValidationErrors
		if errors.As(err, &verr) {
			msgs := make([]string, 0, len(verr))

			labels := map[string]string{
				"Username":    "username",
				"NewPassword": "new password",
			}

			for _, fe := range verr {
				field := fe.StructField()
				lbl, ok := labels[field]
				if !ok {
					lbl = strings.ToLower(field)
				}

				var msg string
				switch fe.Tag() {
				case "required":
					msg = fmt.Sprintf("%s is required", lbl)
				case "min":
					msg = fmt.Sprintf("%s must be at least %s characters", lbl, fe.Param())
				default:
					msg = fmt.Sprintf("%s is invalid", lbl)
				}
				msgs = append(msgs, msg)
			}

			c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: strings.Join(msgs, "; ")})
			return
		}

		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid input"})
		return
	}

	err := h.svc.ForgotPassword(input.Username, input.NewPassword)
	if err != nil {
		if errors.Is(err, application.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, response.MessageResponse{Message: "Password reset successfully"})
}

// GetUsers godoc
// @Summary List all users
// @Tags users
// @Security BearerAuth
// @Produce json
// @Success 200 {array} user.UserWithSuperAdmin
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /users [get]
func (h *UserHandler) GetUsers(c *gin.Context) {
	users, err := h.svc.ListUsers()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, []user.UserWithSuperAdmin{})
			return
		}
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
// @Success 200 {object} response.SuccessResponse{data=[]user.UserWithSuperAdmin}
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /users/paging [get]
func (h *UserHandler) ListUsersPaging(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	users, err := h.svc.ListUserByPaging(page, limit)
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
// @Success 200 {object} user.UserWithSuperAdmin
// @Failure 400 {object} response.ErrorResponse "Invalid user id"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /users/{id} [get]
func (h *UserHandler) GetUserByID(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid user id"})
		return
	}

	user, err := h.svc.FindUserByID(id)
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
// @Success 200 {object} user.UserDTO "Updated user info"
// @Failure 400 {object} response.ErrorResponse "Bad request error"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 500 {object} response.ErrorResponse "Internal server error"
// @Router /users/{id} [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid user id"})
		return
	}

	var input user.UpdateUserInput
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	updatedUser, err := h.svc.UpdateUser(id, input)
	if err != nil {
		switch err {
		case application.ErrUserNotFound:
			c.JSON(http.StatusNotFound, response.ErrorResponse{Error: err.Error()})
		case application.ErrMissingOldPassword, application.ErrIncorrectPassword:
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
func (h *UserHandler) DeleteUser(c *gin.Context) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid user id"})
		return
	}

	if err := h.svc.RemoveUser(id); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
