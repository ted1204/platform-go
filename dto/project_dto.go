package dto

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
