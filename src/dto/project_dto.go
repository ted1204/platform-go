package dto

import "github.com/linskybing/platform-go/src/repositories"

type CreateProjectDTO struct {
	ProjectName string  `form:"project_name" binding:"required"`
	Description *string `form:"description,omitempty"`
	GID         uint    `form:"g_id" binding:"required"`
	GPUQuota    *int    `form:"gpu_quota,omitempty"`
	GPUAccess   *string `form:"gpu_access,omitempty"`
	MPSLimit    *int    `form:"mps_limit,omitempty"`
	MPSMemory   *int    `form:"mps_memory,omitempty"`
}

type UpdateProjectDTO struct {
	ProjectName *string `form:"project_name,omitempty"`
	Description *string `form:"description,omitempty"`
	GID         *uint   `form:"g_id,omitempty"`
	GPUQuota    *int    `form:"gpu_quota,omitempty"`
	GPUAccess   *string `form:"gpu_access,omitempty"`
	MPSLimit    *int    `form:"mps_limit,omitempty"`
	MPSMemory   *int    `form:"mps_memory,omitempty"`
}

type CreateProjectPVCDTO struct {
	Name string `json:"name" binding:"required"`
	Size string `json:"size" binding:"required"`
}

type GIDGetter interface {
	GetGID() uint
}

type GIDByRepoGetter interface {
	GetGIDByRepo(repos *repositories.Repos) uint
}

func (d CreateProjectDTO) GetGID() uint {
	return d.GID
}
