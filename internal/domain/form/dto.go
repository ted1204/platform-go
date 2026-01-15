package form

type CreateFormDTO struct {
	ProjectID   *uint  `json:"project_id"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description" binding:"required"`
	Tag         string `json:"tag"` // TODO: validate against configured tags
}

type UpdateFormStatusDTO struct {
	Status string `json:"status" binding:"required"`
}

type CreateFormMessageDTO struct {
	Content string `json:"content" binding:"required"`
}
