package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/src/response"
	"github.com/linskybing/platform-go/src/utils"
)

// AuthStatusHandler returns the status of the user's token (valid/expired)
func AuthStatusHandler(c *gin.Context) {
	uid, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "token expired"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "valid", "user_id": uid})
}
