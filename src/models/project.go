package models

import "time"

type Project struct {
	PID         uint      `gorm:"primaryKey;column:p_id"`
	ProjectName string    `gorm:"size:100;not null"`
	Description string    `gorm:"type:text"`
	GID         uint      `gorm:"not null"` // foreign key: group_list.g_id
	GPUQuota    int       `gorm:"default:0;column:gpu_quota"`
	GPUAccess   string    `gorm:"default:'shared';column:gpu_access"` // 'none', 'shared', 'dedicated', or comma-separated list e.g. 'shared,dedicated'
	MPSLimit    int       `gorm:"default:100;column:mps_limit"`       // MPS Thread Percentage (0-100)
	MPSMemory   int       `gorm:"default:0;column:mps_memory"`        // MPS Memory Limit in MB (0 = no limit)
	CreatedAt   time.Time `gorm:"column:create_at"`
	UpdatedAt   time.Time `gorm:"column:update_at"`
}
