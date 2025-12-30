package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/response"
	"github.com/linskybing/platform-go/src/services"
	"github.com/linskybing/platform-go/src/types"
	"github.com/linskybing/platform-go/src/utils"
	corev1 "k8s.io/api/core/v1"
)

type K8sHandler struct {
	K8sService     *services.K8sService
	UserService    *services.UserService
	ProjectService *services.ProjectService
}

func NewK8sHandler(K8sService *services.K8sService, UserService *services.UserService, ProjectService *services.ProjectService) *K8sHandler {
	return &K8sHandler{K8sService: K8sService, UserService: UserService, ProjectService: ProjectService}
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

	_, err = h.K8sService.OpenUserGlobalFileBrowser(c, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "Failed to start file browser: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User file browser ready",
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

// UserStorageProxy 處理所有通往 FileBrowser 的流量
// @Router /k8s/user-storage/proxy/*path [all]
func (h *K8sHandler) UserStorageProxy(c *gin.Context) {
	claimsVal, _ := c.Get("claims")
	claims := claimsVal.(*types.Claims)
	userID := claims.UserID
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// 2. 找出該使用者的 Service 內部位址
	// 假設你有一個 Helper function 可以組出 K8s 內部的 DNS 名稱
	// 格式通常是: http://{service-name}.{namespace}.svc.cluster.local:{port}
	user, _ := h.UserService.FindUserByID(userID)
	safeUsername := strings.ToLower(user.Username)

	// 根據我們之前的命名規則
	serviceName := fmt.Sprintf("fb-hub-svc-%s", safeUsername)
	namespace := fmt.Sprintf("user-%s-storage", safeUsername)
	targetStr := fmt.Sprintf("http://%s.%s.svc.cluster.local:80", serviceName, namespace)

	remote, err := url.Parse(targetStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid target url"})
		return
	}

	// 3. 建立反向代理
	proxy := httputil.NewSingleHostReverseProxy(remote)

	// 4. 修改請求路徑 (Path Rewriting)
	// 前端呼叫: /k8s/user-storage/proxy/files/...
	// 後端轉發: /files/... (去除 /k8s/user-storage/proxy 前綴)
	// 注意：FileBrowser 需要設定 baseurl (詳見步驟 3) 才能完美運作，
	// 這裡我們示範標準的 Director 修改
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// 移除 Gin 的路由前綴，讓後面的 FileBrowser 收到正確的路徑
		// 假設你的路由群組是 /k8s/user-storage/proxy
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/k8s/user-storage/proxy")

		// 確保 Header 正確
		req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
		req.Header.Set("X-Forwarded-Proto", "http")
	}

	// 5. 執行代理 (直接接管 ResponseWriter)
	proxy.ServeHTTP(c.Writer, c.Request)
}

// ListProjectStorages retrieves a list of all project-related PVCs.
// @Summary List all project storages
// @Description Fetch all PersistentVolumeClaims that are managed by the platform for projects.
// @Tags K8s/ProjectStorage
// @Accept json
// @Produce json
// @Success 200 {array} dto.ProjectPVCOutput
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /k8s/storage/projects [get]
func (h *K8sHandler) ListProjectStorages(c *gin.Context) {
	// 1. Setup Context with Timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// 2. Call Service
	list, err := h.K8sService.ListAllProjectStorages(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch project storages",
			"details": err.Error(),
		})
		return
	}

	// 3. Return Result
	c.JSON(http.StatusOK, list)
}

// GetUserProjectStorages godoc
// @Summary List storages for projects the user belongs to
// @Description Fetches all PVCs for projects where the current user is a member.
// @Tags k8s
// @Produce json
// @Success 200 {array} dto.ProjectPVCOutput
// @Failure 401 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /k8s/projects/my-storages [get]
func (h *K8sHandler) GetUserProjectStorages(c *gin.Context) {
	// 1. Get UserID from Context
	uid, err := utils.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "Unauthorized"})
		return
	}

	// 2. Fetch projects with Roles using the updated View
	projects, err := h.ProjectService.GetProjectsByUser(uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "Failed to fetch user projects: " + err.Error()})
		return
	}

	// 3. Create a map to store ProjectID -> Role for quick lookup
	userProjectRoles := make(map[uint]string)
	for _, p := range projects {
		userProjectRoles[p.PID] = p.Role
	}

	// 4. Setup Context for K8s operations
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	// 5. Get all storage info from K8s
	allStorages, err := h.K8sService.ListAllProjectStorages(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "Failed to fetch storage status"})
		return
	}

	// 6. Filter and Inject Roles
	var userStorages []dto.ProjectPVCOutput
	for _, s := range allStorages {
		idUint, err := strconv.ParseUint(s.ID, 10, 64)
		if err != nil {
			continue
		}

		// Check if the project ID exists in our role map
		if role, exists := userProjectRoles[uint(idUint)]; exists {
			s.Role = role // English: Inject the role (admin/manager/member) into the output DTO
			userStorages = append(userStorages, s)
		}
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Code:    0,
		Message: "success",
		Data:    userStorages,
	})
}

// CreateProjectStorage provisions a new shared storage (PVC) for a project.
// @Summary Create project storage
// @Description Provisions a Namespace and PVC for the specified project. Auto-generates labels for management.
// @Tags K8s/ProjectStorage
// @Accept json
// @Produce json
// @Param request body dto.CreateProjectStorageRequest true "Project Storage Request"
// @Success 201 {object} map[string]interface{} "Storage created successfully"
// @Failure 400 {object} map[string]string "Invalid request parameters"
// @Failure 409 {object} map[string]string "Storage already exists"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /k8s/storage/projects [post]
func (h *K8sHandler) CreateProjectStorage(c *gin.Context) {
	// 1. Bind JSON Payload
	var req dto.CreateProjectStorageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request parameters",
			"details": err.Error(),
		})
		return
	}

	// 2. Setup Context
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 3. Call Service
	createdPVC, err := h.K8sService.CreateProjectPVC(ctx, req)
	if err != nil {
		// Check for specific errors (e.g., already exists)
		if strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, gin.H{"error": "Storage for this project already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to provision storage",
			"details": err.Error(),
		})
		return
	}

	// 4. Return Success Response
	c.JSON(http.StatusCreated, gin.H{
		"message":   "Project storage created successfully",
		"id":        req.ProjectID,
		"pvcName":   createdPVC.Name,
		"namespace": createdPVC.Namespace,
		"capacity":  req.Capacity,
		"createdAt": createdPVC.CreationTimestamp,
	})
}

// ProjectStorageProxy forwards traffic to the FileBrowser instance of a specific project.
// @Summary Reverse proxy to project file browser
// @Description Proxies requests to the internal K8s Service of the project's FileBrowser. Requires the drive to be started.
// @Tags K8s/ProjectStorage
// @Param id path int true "Project ID"
// @Param path path string true "Path to access (e.g., /files/)"
// @Success 200 {string} string "HTML/Content"
// @Failure 400 {object} map[string]string "Invalid Project ID"
// @Failure 404 {object} map[string]string "Project not found"
// @Failure 502 {object} map[string]string "Storage service unreachable"
// @Router /k8s/storage/projects/{id}/proxy/{path} [get]
// @Router /k8s/storage/projects/{id}/proxy/{path} [post]
// @Router /k8s/storage/projects/{id}/proxy/{path} [put]
// @Router /k8s/storage/projects/{id}/proxy/{path} [delete]
func (h *K8sHandler) ProjectStorageProxy(c *gin.Context) {
	// 1. Get Project ID from URL
	projectIDStr := c.Param("id")
	projectID, err := strconv.ParseUint(projectIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Project ID"})
		return
	}

	// 2. Fetch Project Details to reconstruct Namespace name
	// We need the Project Name to recreate the hashed namespace via Utils.
	project, err := h.ProjectService.GetProject(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	// 1. Reconstruct Namespace (Matches your previous logic)
	targetNamespace := utils.GenerateSafeResourceName("project", project.ProjectName, project.PID)

	// 2. Reconstruct Service Name
	// English Comment: Based on your kubectl output, the service name follows the pattern:
	// "filebrowser-" + pvcName + "-svc"
	// Assuming your PVC name follows: "pvc-" + targetNamespace
	pvcName := fmt.Sprintf("pvc-%s", targetNamespace)
	serviceName := fmt.Sprintf("filebrowser-%s-svc", pvcName)

	// 3. Construct the internal K8s Cluster DNS URL
	// targetURL will now be: http://filebrowser-pvc-project-test-b1f7f5-svc.project-test-b1f7f5.svc.cluster.local:80
	targetURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:80", serviceName, targetNamespace)

	remote, err := url.Parse(targetURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid target URL configuration"})
		return
	}

	// 4. Setup Reverse Proxy
	proxy := httputil.NewSingleHostReverseProxy(remote)

	// 5. Rewrite Path (Director)
	// The path sent to K8s should not contain the proxy prefix.
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// English Comment: Set headers so FileBrowser understands its location
		req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
		req.Header.Set("X-Forwarded-Prefix", fmt.Sprintf("/k8s/storage/projects/%d/proxy", projectID))
		req.Header.Set("X-Forwarded-Proto", "http")
	}

	// 6. Error Handler for Proxy
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		// This usually happens if the Pod is not running or Service is unreachable
		fmt.Printf("[Proxy Error] Target: %s, Error: %v\n", targetURL, err)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(fmt.Sprintf(`{"error": "Storage service unreachable. Is the drive started?", "details": "%v"}`, err)))
	}

	// 7. Serve Content
	proxy.ServeHTTP(c.Writer, c.Request)
}

// StartProjectFileBrowser godoc
// @Summary Start project file browser with Group Role RBAC
// @Description Users with 'admin' or 'manager' roles in the project's owning group get RW access.
// @Tags k8s
// @Router /k8s/storage/projects/{id}/start [post]
func (h *K8sHandler) StartProjectFileBrowser(c *gin.Context) {
	pIDStr := c.Param("id")
	pID, _ := strconv.ParseUint(pIDStr, 10, 64)
	uID, _ := utils.GetUserIDFromContext(c)

	// 1. Determine user's role based on Group ownership of the project
	role, err := h.ProjectService.GetUserRoleInProjectGroup(uID, uint(pID))
	if err != nil {
		c.JSON(http.StatusForbidden, response.ErrorResponse{Error: "Access denied: role not found"})
		return
	}

	// 2. Permission Logic: Only higher roles get Write access
	isReadOnly := !(role == "admin" || role == "manager")

	// 3. Metadata for K8s
	project, _ := h.ProjectService.GetProject(uint(pID))
	targetNamespace := utils.GenerateSafeResourceName("project", project.ProjectName, project.PID)
	pvcName := fmt.Sprintf("pvc-%s", targetNamespace)

	baseURL := fmt.Sprintf("/k8s/storage/projects/%d/proxy", pID)
	// 4. Start FileBrowser with the calculated readOnly flag
	_, err = h.K8sService.StartFileBrowser(c.Request.Context(), targetNamespace, pvcName, isReadOnly, baseURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"role":     role,
			"readOnly": isReadOnly,
		},
	})
}

// StopProjectFileBrowser godoc
// @Summary Stop File Browser for a specific project
// @Description Terminates the FileBrowser pod associated with the project.
// @Tags k8s
// @Produce json
// @Param id path int true "Project ID"
// @Success 200 {object} response.SuccessResponse
// @Router /k8s/storage/projects/{id}/stop [delete]
func (h *K8sHandler) StopProjectFileBrowser(c *gin.Context) {
	projectID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	project, err := h.ProjectService.GetProject(uint(projectID))
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorResponse{Error: "Project not found"})
		return
	}

	targetNamespace := utils.GenerateSafeResourceName("project", project.ProjectName, project.PID)
	pvcName := fmt.Sprintf("pvc-%s", targetNamespace)

	err = h.K8sService.StopFileBrowser(c.Request.Context(), targetNamespace, pvcName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.SuccessResponse{
		Code:    0,
		Message: "Project File Browser stopped",
	})
}
