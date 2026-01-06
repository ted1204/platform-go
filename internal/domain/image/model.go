package image

import "gorm.io/gorm"

// ImageRequest is a user-submitted request to allow/pull an image.
type ImageRequest struct {
	gorm.Model
	UserID    uint   `json:"user_id"`
	ProjectID *uint  `json:"project_id"` // nil for global request
	Name      string `json:"name"`       // e.g. registry/project/repo
	Tag       string `json:"tag"`        // e.g. v1.2.3
	Status    string `json:"status"`     // pending/approved/rejected
	Note      string `json:"note"`       // admin note
}

// AllowedImage stores images approved for use.
// IsGlobal=true means admin-approved and visible to all
// IsGlobal=false means project-scoped, only visible to that project
type AllowedImage struct {
	gorm.Model
	Name      string `json:"name"` // e.g. registry/project/repo
	Tag       string `json:"tag"`
	ProjectID *uint  `json:"project_id"`                     // nil if global
	IsGlobal  bool   `json:"is_global" gorm:"default:false"` // true if admin-approved for all
	IsPulled  bool   `json:"is_pulled" gorm:"default:false"` // true if image has been pulled to cluster
	CreatedBy uint   `json:"created_by"`                     // user who added this image
}
