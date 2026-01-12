package image

type CreateImageRequestDTO struct {
	Registry  string `json:"registry"`
	ImageName string `json:"image_name" binding:"required"`
	Tag       string `json:"tag" binding:"required"`
	ProjectID *uint  `json:"project_id"`
}

type UpdateImageRequestDTO struct {
	Status string `json:"status" binding:"required,oneof=approved rejected"`
	Note   string `json:"note"`
}

type AllowedImageDTO struct {
	ID        uint   `json:"id"`
	Registry  string `json:"registry"`
	ImageName string `json:"image_name"`
	Tag       string `json:"tag"`
	Digest    string `json:"digest"`
	ProjectID *uint  `json:"project_id"`
	IsGlobal  bool   `json:"is_global"`
	IsPulled  bool   `json:"is_pulled"`
}
