package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/response"
	"github.com/linskybing/platform-go/src/services"
	"github.com/linskybing/platform-go/src/utils"
	corev1 "k8s.io/api/core/v1"
)

type K8sHandler struct {
	svc *services.K8sService
}

func NewK8sHandler(svc *services.K8sService) *K8sHandler {
	return &K8sHandler{svc: svc}
}

// @Summary Create a Kubernetes Job
// @Tags k8s
// @Accept json
// @Produce json
// @Param body body dto.CreateJobDTO true "Job Specification"
// @Success 201 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /k8s/jobs [post]
func (h *K8sHandler) CreateJob(c *gin.Context) {
	var input dto.CreateJobDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	uid, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Unauthorized"})
		return
	}

	if err := h.svc.CreateJob(c.Request.Context(), uid, input); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response.SuccessResponse{
		Code:    0,
		Message: "Job created successfully",
	})
}

// @Summary Get single PVC
// @Tags k8s
// @Produce json
// @Param namespace path string true "namespace"
// @Param name path string true "PVC name"
// @Success 200 {object} response.SuccessResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /k8s/pvc/{namespace}/{name} [get]
func (h *K8sHandler) GetPVC(c *gin.Context) {
	ns := c.Param("namespace")
	name := c.Param("name")

	pvc, err := h.svc.GetPVC(ns, name)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: err.Error()})
		return
	}

	size := ""
	if q, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		size = q.String()
	}

	pvcDTO := dto.PVC{
		Name:      pvc.Name,
		Namespace: pvc.Namespace,
		Status:    string(pvc.Status.Phase),
		Size:      size,
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Code:    0,
		Message: "success",
		Data:    pvcDTO,
	})
}

// @Summary List all PVC in Namespace
// @Tags k8s
// @Produce json
// @Param namespace path string true "namespace"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /k8s/pvc/list/{namespace} [get]
func (h *K8sHandler) ListPVCs(c *gin.Context) {
	ns := c.Param("namespace")

	pvcs, err := h.svc.ListPVCs(ns)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	var pvcDTOs []dto.PVC
	for _, pvc := range pvcs {
		size := ""
		if q, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			size = q.String()
		}
		pvcDTOs = append(pvcDTOs, dto.PVC{
			Name:      pvc.Name,
			Namespace: pvc.Namespace,
			Status:    string(pvc.Status.Phase),
			Size:      size,
		})
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Code:    0,
		Message: "success",
		Data:    pvcDTOs,
	})
}

// @Summary List all PVC in Project
// @Tags k8s
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /k8s/pvc/by-project/{id} [get]
func (h *K8sHandler) ListPVCsByProject(c *gin.Context) {
	pid := c.Param("id")

	username, err := utils.GetUserNameFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Unauthorized"})
		return
	}

	ns := fmt.Sprintf("proj-%s-%s", pid, username)

	pvcs, err := h.svc.ListPVCs(ns)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	var pvcDTOs []dto.PVC
	for _, pvc := range pvcs {
		size := ""
		if q, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			size = q.String()
		}
		pvcDTOs = append(pvcDTOs, dto.PVC{
			Name:      pvc.Name,
			Namespace: pvc.Namespace,
			Status:    string(pvc.Status.Phase),
			Size:      size,
		})
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Code:    0,
		Message: "success",
		Data:    pvcDTOs,
	})
}

// @Summary Create PVC
// @Tags k8s
// @Accept x-www-form-urlencoded
// @Produce json
// @Param namespace formData string true "namespace"
// @Param name formData string true "PVC name"
// @Param storageClassName formData string true "Storage Class Name"
// @Param size formData string true "Size (e.g. 1Gi)"
// @Success 201 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /k8s/pvc [post]
func (h *K8sHandler) CreatePVC(c *gin.Context) {
	var input dto.CreatePVCDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	if err := h.svc.CreatePVC(input); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, response.SuccessResponse{
		Code:    0,
		Message: "PVC created",
	})
}

// @Summary Expand PVC
// @Tags k8s
// @Accept x-www-form-urlencoded
// @Produce json
// @Param namespace formData string true "namespace"
// @Param name formData string true "PVC name"
// @Param size formData string true "New Size (e.g. 2Gi)"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /k8s/pvc/expand [put]
func (h *K8sHandler) ExpandPVC(c *gin.Context) {
	var input dto.ExpandPVCDTO
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	if err := h.svc.ExpandPVC(input); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.SuccessResponse{
		Code:    0,
		Message: "PVC expanded",
	})
}

// @Summary Delete PVC
// @Tags k8s
// @Produce json
// @Param namespace path string true "namespace"
// @Param name path string true "PVC name"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /k8s/pvc/{namespace}/{name} [delete]
func (h *K8sHandler) DeletePVC(c *gin.Context) {
	ns := c.Param("namespace")
	name := c.Param("name")

	if err := h.svc.DeletePVC(ns, name); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.SuccessResponse{
		Code:    0,
		Message: "PVC deleted",
	})
}

// @Summary Start File Browser
// @Tags k8s
// @Accept json
// @Produce json
// @Param body body dto.StartFileBrowserDTO true "Start Info"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /k8s/filebrowser/start [post]
func (h *K8sHandler) StartFileBrowser(c *gin.Context) {
	var input dto.StartFileBrowserDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	nodePort, err := h.svc.StartFileBrowser(c.Request.Context(), input.Namespace, input.PVCName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Code:    0,
		Message: "File Browser Started",
		Data:    gin.H{"nodePort": nodePort},
	})
}

// @Summary Stop File Browser
// @Tags k8s
// @Accept json
// @Produce json
// @Param body body dto.StopFileBrowserDTO true "Stop Info"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /k8s/filebrowser/stop [post]
func (h *K8sHandler) StopFileBrowser(c *gin.Context) {
	var input dto.StopFileBrowserDTO
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.svc.StopFileBrowser(c.Request.Context(), input.Namespace, input.PVCName); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Code:    0,
		Message: "File Browser Stopped",
	})
}
