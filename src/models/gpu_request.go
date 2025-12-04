package models

import "time"

type GPURequestStatus string

const (
	GPURequestStatusPending  GPURequestStatus = "pending"
	GPURequestStatusApproved GPURequestStatus = "approved"
	GPURequestStatusRejected GPURequestStatus = "rejected"
)

type GPURequestType string

const (
	GPURequestTypeQuota  GPURequestType = "quota"
	GPURequestTypeAccess GPURequestType = "access"
)

type GPURequest struct {
	ID                  uint             `gorm:"primaryKey;column:id"`
	ProjectID           uint             `gorm:"not null;column:project_id"`
	RequesterID         uint             `gorm:"not null;column:requester_id"`
	Type                GPURequestType   `gorm:"not null;type:varchar(20);column:type"`
	RequestedQuota      int              `gorm:"default:0;column:requested_quota"`
	RequestedAccessType string           `gorm:"size:20;column:requested_access_type"`
	Reason              string           `gorm:"type:text;column:reason"`
	Status              GPURequestStatus `gorm:"default:'pending';type:varchar(20);column:status"`
	CreatedAt           time.Time        `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt           time.Time        `gorm:"column:updated_at;autoUpdateTime"`
}
