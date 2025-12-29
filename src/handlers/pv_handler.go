package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/src/config"
	"github.com/linskybing/platform-go/src/response"
	"github.com/linskybing/platform-go/src/utils"
)

// CreateSharedPVHandler creates a shared PersistentVolume and PersistentVolumeClaim for a project (admin only).
// @Summary Create shared PV and PVC (Admin)
// @Description Admin creates a shared PersistentVolume and PersistentVolumeClaim for a project (RWX, for project sharing)
// @Tags admin,pv
// @Accept json
// @Produce json
// @Param body body struct{ProjectID uint `json:"project_id"`; PVName string `json:"pv_name"`; Size string `json:"size"`} true "PV/PVC creation info"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /admin/pv [post]
func CreateSharedPVHandler(c *gin.Context) {
	// Only admin should access this handler (ensure via router/middleware)
	type Req struct {
		ProjectID uint   `json:"project_id" binding:"required"`
		PVName    string `json:"pv_name" binding:"required"`
		Size      string `json:"size" binding:"required"`
	}
	var req Req
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}
	// Create PV
	err := utils.CreatePV(req.PVName, config.DefaultStorageClassName, req.Size, req.PVName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "Failed to create PV: " + err.Error()})
		return
	}
	// Create PVC bound to PV in project namespace
	ns := utils.FormatProjectNamespace(req.ProjectID)
	err = utils.CreateBoundPVC(ns, req.PVName, config.DefaultStorageClassName, req.Size, req.PVName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "Failed to create PVC: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Shared PV/PVC created successfully"})
}

// ExpandPVHandler expands a PersistentVolumeClaim (PVC) in a given namespace (admin only).
// @Summary Expand PVC (Admin)
// @Description Admin expands a PersistentVolumeClaim (PVC) in the specified namespace. PV will be expanded if storage class supports it.
// @Tags admin,pv
// @Accept json
// @Produce json
// @Param namespace query string true "Namespace of the PVC"
// @Param body body struct{PVName string `json:"pv_name"`; NewSize string `json:"new_size"`} true "PVC expansion info"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /admin/pv/expand [post]
func ExpandPVHandler(c *gin.Context) {
	// Only admin should access this handler (ensure via router/middleware)
	type Req struct {
		PVName  string `json:"pv_name" binding:"required"`
		NewSize string `json:"new_size" binding:"required"`
	}
	var req Req
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}
	ns := c.Query("namespace")
	if ns == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "namespace is required"})
		return
	}
	err := utils.ExpandPVC(ns, req.PVName, req.NewSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "Failed to expand PV: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "PV expanded successfully"})
}
