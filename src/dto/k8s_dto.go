package dto

type CreateJobDTO struct {
	Name        string   `json:"name" binding:"required"`
	Namespace   string   `json:"namespace" binding:"required"`
	Image       string   `json:"image" binding:"required"`
	Command     []string `json:"command" binding:"required"`
	Priority    string   `json:"priority"` // "high" or "low"
	GPUCount    int      `json:"gpu_count"`
	GPUType     string   `json:"gpu_type"` // "dedicated" or "shared"
	Parallelism int32    `json:"parallelism"`
	Completions int32    `json:"completions"`
	Volumes     []Volume `json:"volumes"`
}

type Volume struct {
	Name      string `json:"name"`
	PVCName   string `json:"pvc_name"`
	MountPath string `json:"mount_path"`
}

type CreatePVCDTO struct {
	Namespace        string `form:"namespace" binding:"required"`
	Name             string `form:"name" binding:"required"`
	StorageClassName string `form:"storageClassName" binding:"required"`
	Size             string `form:"size" binding:"required"`
}

type ExpandPVCDTO struct {
	Namespace string `form:"namespace" binding:"required"`
	Name      string `form:"name" binding:"required"`
	Size      string `form:"size" binding:"required"`
}

type ExpandStorageInput struct {
	NewSize string `json:"new_size" binding:"required" example:"1Ti"`
}
