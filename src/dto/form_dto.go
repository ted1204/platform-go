package dto

type CreateFormDTO struct {
	ProjectID   *uint  `json:"project_id"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description" binding:"required"`
}

type UpdateFormStatusDTO struct {
	Status string `json:"status" binding:"required"`
}
