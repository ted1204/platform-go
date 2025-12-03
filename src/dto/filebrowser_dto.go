package dto

type StartFileBrowserDTO struct {
	Namespace string `json:"namespace" binding:"required"`
	PVCName   string `json:"pvc_name" binding:"required"`
}

type StopFileBrowserDTO struct {
	Namespace string `json:"namespace" binding:"required"`
	PVCName   string `json:"pvc_name" binding:"required"`
}
