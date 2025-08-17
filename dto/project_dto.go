package dto

import "github.com/linskybing/platform-go/repositories"

type CreateProjectDTO struct {
	ProjectName string  `form:"project_name" binding:"required"`
	Description *string `form:"description,omitempty"`
	GID         uint    `form:"g_id" binding:"required"`
}

type UpdateProjectDTO struct {
	ProjectName *string `form:"project_name,omitempty"`
	Description *string `form:"description,omitempty"`
	GID         *uint   `form:"g_id,omitempty"`
}

type GIDGetter interface {
	GetGID() uint
	GetGIDByRepo(repos *repositories.Repos) uint
}

func (d CreateProjectDTO) GetGID() uint {
	return d.GID
}
