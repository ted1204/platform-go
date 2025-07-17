package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/models"
	"golang.org/x/crypto/bcrypt"
)

// GET /users
func GetUsers(c *gin.Context) {
	var users []models.UserWithSuperAdmin
	if err := db.DB.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, users)
}

// GET /users/:id
func GetUserByID(c *gin.Context) {
	idStr := c.Param("id")
	idUint64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	id := uint(idUint64)

	var user models.UserWithSuperAdmin
	if err := db.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// UpdateUser updates the information of a user by ID.
// @Summary Update user
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
// @Failure 400 {object} map[string]string "Bad request error"
// @Failure 404 {object} map[string]string "User not found"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /users/{id} [put]
func UpdateUser(c *gin.Context) {
	idParam := c.Param("id")
	targetUserID64, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user id"})
		return
	}
	targetUserID := uint(targetUserID64)

	var user models.User
	if err := db.DB.First(&user, targetUserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var input dto.UpdateUserInput
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.Password != nil {
		if input.OldPassword == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Old password is required to change password"})
			return
		}
		err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(*input.OldPassword))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Old password is incorrect"})
			return
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash new password"})
			return
		}
		user.Password = string(hashed)
	}

	if input.Type != nil {
		user.Type = *input.Type
	}

	if input.Status != nil {
		user.Status = *input.Status
	}

	if input.Email != nil {
		user.Email = input.Email
	}

	if input.FullName != nil {
		user.FullName = input.FullName
	}

	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// DELETE /users/:id
func DeleteUser(c *gin.Context) {
	idParam := c.Param("id")
	targetUserID64, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user id"})
		return
	}
	targetUserID := uint(targetUserID64)

	if err := db.DB.Delete(&models.User{}, targetUserID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
