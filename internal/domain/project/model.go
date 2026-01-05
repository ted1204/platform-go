package project

import "time"

// GPUAccessType defines the type of GPU access allowed for a project
type GPUAccessType string

const (
	GPUAccessNone      GPUAccessType = "none"      // No GPU access
	GPUAccessShared    GPUAccessType = "shared"    // Shared GPU via MPS
	GPUAccessDedicated GPUAccessType = "dedicated" // Dedicated GPU
)

// Project represents a user project with resource quotas
type Project struct {
	PID         uint      `gorm:"primaryKey;column:p_id;autoIncrement"`
	ProjectName string    `gorm:"size:100;not null"`
	Description string    `gorm:"type:text"`
	GID         uint      `gorm:"not null"`                   // Group ID
	GPUQuota    int       `gorm:"default:0;column:gpu_quota"` // GPU quota in integer units (system auto-injects CUDA_MPS_ACTIVE_THREAD_PERCENTAGE)
	GPUAccess   string    `gorm:"default:'shared';column:gpu_access"`
	MPSMemory   int       `gorm:"default:0;column:mps_memory"` // MPS memory limit in MB (optional)
	CreatedAt   time.Time `gorm:"column:create_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:update_at;autoUpdateTime"`
}

// TableName specifies the database table name
func (Project) TableName() string {
	return "project_list"
}

// CanUseDedicatedGPU checks if project can use dedicated GPU
func (p *Project) CanUseDedicatedGPU() bool {
	return p.hasAccessType(GPUAccessDedicated)
}

// CanUseMPS checks if project can use MPS GPU sharing
func (p *Project) CanUseMPS() bool {
	return p.hasAccessType(GPUAccessShared)
}

// hasAccessType checks if project has specific GPU access type
func (p *Project) hasAccessType(accessType GPUAccessType) bool {
	access := p.GPUAccess
	if access == string(accessType) {
		return true
	}
	// Check if it's in CSV format
	for i := 0; i < len(access); {
		end := i
		for end < len(access) && access[end] != ',' {
			end++
		}
		if access[i:end] == string(accessType) {
			return true
		}
		i = end + 1
	}
	return false
}

// HasGPUQuota checks if project has GPU quota available
func (p *Project) HasGPUQuota() bool {
	return p.GPUQuota > 0
}

// GetMPSUnits converts GPU quota to MPS units (1 dedicated GPU = 10 MPS units)
func (p *Project) GetMPSUnits() int {
	return p.GPUQuota * 10
}
