package dto

type CreateTicketDTO struct {
	ProjectID   *uint  `json:"project_id"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description" binding:"required"`
}

type UpdateTicketStatusDTO struct {
	Status string `json:"status" binding:"required"`
}
