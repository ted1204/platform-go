// src/dto/storage_dto.go
package dto

import "time"

// CreateProjectStorageRequest defines the payload for creating project storage.
// 定義建立專案儲存空間的請求參數
type CreateProjectStorageRequest struct {
	ProjectID   uint   `json:"projectId" binding:"required"`
	ProjectName string `json:"projectName" binding:"required"`
	Capacity    int    `json:"capacity" binding:"required,min=1"` // In Gi
}

// ProjectPVCOutput defines the response structure for listing storages.
type ProjectPVCOutput struct {
	ID          string    `json:"id"`          // The Project ID (string format to prevent frontend conversion issues)
	PVCName     string    `json:"pvcName"`     // The K8s PVC Name
	ProjectName string    `json:"projectName"` // Human readable name
	Namespace   string    `json:"namespace"`   // K8s Namespace
	Capacity    string    `json:"capacity"`    // e.g., "10Gi"
	Status      string    `json:"status"`      // e.g., "Bound"
	Role        string    `json:"role"`        // [NEW] User's role in the group (admin/manager/member)
	AccessMode  string    `json:"accessmode"`
	CreatedAt   time.Time `json:"createdAt"` // Creation timestamp
}
