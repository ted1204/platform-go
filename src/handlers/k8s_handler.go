package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/response"
	"github.com/linskybing/platform-go/src/services"
	"github.com/linskybing/platform-go/src/types"
	"github.com/linskybing/platform-go/src/utils"
	corev1 "k8s.io/api/core/v1"
)

type K8sHandler struct {
	K8sService  *services.K8sService
	UserService *services.UserService
}

func NewK8sHandler(K8sService *services.K8sService, UserService *services.UserService) *K8sHandler {
	return &K8sHandler{K8sService: K8sService, UserService: UserService}
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

	if err := h.K8sService.CreateJob(c.Request.Context(), uid, input); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response.SuccessResponse{
		Code:    0,
		Message: "Job created successfully",
	})
}

// @Summary List Jobs
// @Tags k8s
// @Produce json
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /k8s/jobs [get]
func (h *K8sHandler) ListJobs(c *gin.Context) {
	uid, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Unauthorized"})
		return
	}

	// Check if user is admin (optional, depending on requirements)
	// For now, let's assume we pass isAdmin=false unless we check roles
	// But wait, GetUserIDFromContext only gives ID.
	// We might need to fetch user to check role, or rely on middleware.
	// Let's assume we want to list jobs for the current user.
	// If we want admin to see all, we need to check role.
	// Let's just list by user for now.

	// Actually, let's check if we can get role from context or token.
	// The middleware sets "userID".
	// Let's just list for the user.

	jobs, err := h.K8sService.ListJobs(uid, false) // false for isAdmin for now
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Code:    0,
		Message: "success",
		Data:    jobs,
	})
}

// @Summary Get Job
// @Tags k8s
// @Produce json
// @Param id path int true "Job ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /k8s/jobs/{id} [get]
func (h *K8sHandler) GetJob(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid ID"})
		return
	}

	job, err := h.K8sService.GetJob(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Code:    0,
		Message: "success",
		Data:    job,
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

	pvc, err := h.K8sService.GetPVC(ns, name)
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

	pvcs, err := h.K8sService.ListPVCs(ns)
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

	pvcs, err := h.K8sService.ListPVCs(ns)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	globalPVCName := fmt.Sprintf("user-%s-pv", username)

	var pvcDTOs []dto.PVC
	for _, pvc := range pvcs {
		size := ""
		if q, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			size = q.String()
		}
		isGlobal := pvc.Name == globalPVCName
		pvcDTOs = append(pvcDTOs, dto.PVC{
			Name:      pvc.Name,
			Namespace: pvc.Namespace,
			Status:    string(pvc.Status.Phase),
			Size:      size,
			IsGlobal:  isGlobal,
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

	if err := h.K8sService.CreatePVC(input); err != nil {
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

	if err := h.K8sService.ExpandPVC(input); err != nil {
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

	if err := h.K8sService.DeletePVC(ns, name); err != nil {
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

	nodePort, err := h.K8sService.StartFileBrowser(c.Request.Context(), input.Namespace, input.PVCName)
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

	if err := h.K8sService.StopFileBrowser(c.Request.Context(), input.Namespace, input.PVCName); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Code:    0,
		Message: "File Browser Stopped",
	})
}

// GetUserStorageStatus godoc
// @Summary Check if user storage exists
// @Tags k8s
// @Param username path string true "Username"
// @Success 200 {object} map[string]bool "exists: true/false"
// @Router /k8s/users/{username}/storage/status [get]
func (h *K8sHandler) GetUserStorageStatus(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Username is required"})
		return
	}

	exists, err := h.K8sService.CheckUserStorageExists(c, username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"exists": exists,
	})
}

// InitializeStorage godoc
// @Summary Manually initialize user storage
// @Description Force creation of K8s storage resources (Namespace, PVC, NFS) for a specific user. Useful for recovery.
// @Tags admin
// @Accept json
// @Produce json
// @Param username path string true "Username to initialize"
// @Success 200 {object} response.MessageResponse "Storage initialized successfully"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 500 {object} response.ErrorResponse "Failed to initialize storage"
// @Router /k8s/users/{username}/storage/init [post]
func (h *K8sHandler) InitializeUserStorage(c *gin.Context) {
	username := c.Param("username")

	err := h.K8sService.InitializeUserStorageHub(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: fmt.Sprintf("Failed to init storage: %v", err)})
		return
	}

	c.JSON(http.StatusOK, response.MessageResponse{Message: "Storage initialized successfully"})
}

// ExpandUserStorage godoc
// @Summary Expand user storage capacity
// @Description Increases the size of the underlying PVC for a specific user's storage hub.
// @Tags k8s
// @Accept json
// @Produce json
// @Param username path string true "Target Username"
// @Param input body dto.ExpandStorageInput true "Expansion details"
// @Success 200 {object} response.MessageResponse "Storage expanded successfully"
// @Failure 400 {object} response.ErrorResponse "Invalid input"
// @Failure 500 {object} response.ErrorResponse "Internal Server Error"
// @Router /k8s/users/{username}/storage/expand [put]
func (h *K8sHandler) ExpandUserStorage(c *gin.Context) {
	// 1. Retrieve the target username from the URL path parameter.
	targetUsername := c.Param("username")
	if targetUsername == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Username is required"})
		return
	}

	// 2. Bind JSON payload to get the new size.
	var input dto.ExpandStorageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Invalid input: " + err.Error()})
		return
	}

	// 3. Call the service to perform the expansion.
	err := h.K8sService.ExpandUserStorageHub(targetUsername, input.NewSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "Failed to expand storage: " + err.Error()})
		return
	}

	// 4. Return success response.
	c.JSON(http.StatusOK, response.MessageResponse{
		Message: fmt.Sprintf("Storage for user '%s' expanded to %s successfully", targetUsername, input.NewSize),
	})
}

// OpenMyDrive godoc
// @Summary Open user's global file browser
// @Description Spins up a temporary FileBrowser pod connected to the user's storage hub (NFS Client).
// @Tags user
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Returns nodePort"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 500 {object} response.ErrorResponse "Internal Server Error"
// @Router /k8s/user-storage/browse [post]
func (h *K8sHandler) OpenMyDrive(c *gin.Context) {
	claimsVal, _ := c.Get("claims")
	claims := claimsVal.(*types.Claims)
	userID := claims.UserID
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Unauthorized"})
		return
	}

	user, err := h.UserService.FindUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "User not found: " + err.Error()})
		return
	}

	port, err := h.K8sService.OpenUserGlobalFileBrowser(c, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "Failed to start file browser: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"nodePort": port,
		"message":  "User file browser ready",
	})
}

// StopMyDrive godoc
// @Summary Stop user's global file browser
// @Description Terminates the temporary FileBrowser pod and service for the user's storage hub.
// @Tags user
// @Accept json
// @Produce json
// @Success 200 {object} response.MessageResponse "Resources cleaned up"
// @Failure 401 {object} response.ErrorResponse "Unauthorized"
// @Failure 404 {object} response.ErrorResponse "User not found"
// @Failure 500 {object} response.ErrorResponse "Internal Server Error"
// @Router /k8s/user-storage/browse [delete]
func (h *K8sHandler) StopMyDrive(c *gin.Context) {
	claimsVal, _ := c.Get("claims")
	claims := claimsVal.(*types.Claims)
	userID := claims.UserID
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Unauthorized"})
		return
	}

	user, err := h.UserService.FindUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "User not found: " + err.Error()})
		return
	}

	err = h.K8sService.StopUserGlobalFileBrowser(c, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "Failed to stop file browser: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.MessageResponse{
		Message: "User file browser stopped successfully",
	})
}

// DeleteUserStorage handles the deletion of a user's storage hub resources.
func (h *K8sHandler) DeleteUserStorage(c *gin.Context) {
	targetUsername := c.Param("username")
	if targetUsername == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "Username is required"})
		return
	}

	// Call service to remove Namespace, PVC, and NFS deployments
	err := h.K8sService.DeleteUserStorageHub(c, targetUsername)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "Failed to delete storage: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.MessageResponse{
		Message: fmt.Sprintf("Storage for user '%s' has been completely removed", targetUsername),
	})
}
