package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/src/services"
)

type AdminHandler struct {
	UserService *services.UserService
	K8sService  *services.K8sService
}

func NewAdminHandler(userService *services.UserService) *AdminHandler {
	return &AdminHandler{UserService: userService}
}

// POST /admin/ensure-user-pv
// @Summary      Ensure all users have PV/PVC
// @Tags         admin
// @Produce      json
// @Success      200 {object} map[string]int "created count"
// @Failure      500 {object} map[string]string "error"
// @Router       /admin/ensure-user-pv [post]
func (h *AdminHandler) EnsureAllUserPV(c *gin.Context) {
	count, err := h.UserService.EnsureAllUserPV()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"created": count})
}
