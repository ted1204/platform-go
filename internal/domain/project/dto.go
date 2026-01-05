package project

type CreateProjectDTO struct {
	ProjectName string  `json:"project_name" form:"project_name" binding:"required"`
	Description *string `json:"description,omitempty" form:"description,omitempty"`
	GID         uint    `json:"gid" form:"g_id" binding:"required"`
	GPUQuota    *int    `json:"gpu_quota,omitempty" form:"gpu_quota,omitempty"` // GPU quota in integer units
	GPUAccess   *string `json:"gpu_access,omitempty" form:"gpu_access,omitempty"`
	MPSMemory   *int    `json:"mps_memory,omitempty" form:"mps_memory,omitempty"` // MPS memory limit in MB (optional)
}

type UpdateProjectDTO struct {
	ProjectName *string `json:"project_name,omitempty" form:"project_name,omitempty"`
	Description *string `json:"description,omitempty" form:"description,omitempty"`
	GID         *uint   `json:"gid,omitempty" form:"g_id,omitempty"`
	GPUQuota    *int    `json:"gpu_quota,omitempty" form:"gpu_quota,omitempty"` // GPU quota in integer units
	GPUAccess   *string `json:"gpu_access,omitempty" form:"gpu_access,omitempty"`
	MPSMemory   *int    `json:"mps_memory,omitempty" form:"mps_memory,omitempty"` // MPS memory limit in MB (optional)
}

type CreateProjectPVCDTO struct {
	Name string `json:"name" binding:"required"`
	Size string `json:"size" binding:"required"`
}

type GIDGetter interface {
	GetGID() uint
}

func (d CreateProjectDTO) GetGID() uint {
	return d.GID
}
