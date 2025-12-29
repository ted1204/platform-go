// src/dto/storage_dto.go
package dto

type ApproveStorageRequest struct {
	ProjectID uint   `json:"project_id" binding:"required" example:"101"`
	Size      string `json:"size" binding:"required" example:"10Gi"`
	// 如果需要支援 Hub-Spoke 架構，可以加這個欄位指定來源使用者的 Hub
	// SourceUser string `json:"source_user" example:"sky"`
}
