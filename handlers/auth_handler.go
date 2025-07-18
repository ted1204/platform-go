package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/response"
	"github.com/linskybing/platform-go/services"
)

// Register godoc
// @Summary User registration
// @Tags auth
// @Accept x-www-form-urlencoded
// @Produce json
// @Param input body dto.CreateUserInput true "User registration info"
// @Success 201 {object} response.MessageResponse "User registered successfully"
// @Failure 400 {object} response.ErrorResponse "Invalid input"
// @Failure 409 {object} response.ErrorResponse "Username already taken"
// @Failure 500 {object} response.ErrorResponse "Failed to create user"
// @Router /register [post]
func Register(c *gin.Context) {
	var input dto.CreateUserInput

	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid input"})
		return
	}

	err := services.RegisterUser(input)
	if err != nil {
		if errors.Is(err, services.ErrUsernameTaken) {
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
func Login(c *gin.Context) {
	var req struct {
		Username string `form:"username" binding:"required"`
		Password string `form:"password" binding:"required"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid input"})
		return
	}

	user, token, isAdmin, err := services.LoginUser(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Invalid username or password"})
		return
	}

	c.JSON(http.StatusOK, response.TokenResponse{
		Token:    token,
		UID:      user.UID,
		Username: user.Username,
		IsAdmin:  isAdmin,
	})
}
