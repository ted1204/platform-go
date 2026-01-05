package image

// AllowedImageDTO represents allowed image information
type AllowedImageDTO struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Tag       string `json:"tag"`
	ProjectID *uint  `json:"project_id"`
	IsGlobal  bool   `json:"is_global"`
}

// ImageRequestDTO represents a request to add an image
type ImageRequestDTO struct {
	Name      string `json:"name" binding:"required"`
	Tag       string `json:"tag" binding:"required"`
	ProjectID *uint  `json:"project_id"` // nil for global request
}

// CreateProjectImageDTO represents adding image directly to project
type CreateProjectImageDTO struct {
	Name string `json:"name" binding:"required"`
	Tag  string `json:"tag" binding:"required"`
}

// UpdateImageRequestDTO represents updating an image request
type UpdateImageRequestDTO struct {
	Status string `json:"status" binding:"required,oneof=approved rejected"`
	Note   string `json:"note"`
}

// HarborImageResponse represents images from Harbor API
type HarborImageResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}
