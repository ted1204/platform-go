package image

// Repository defines image-related database operations
type Repository interface {
	CreateRequest(req *ImageRequest) error
	FindRequestByID(id uint) (*ImageRequest, error)
	FindAllRequests() ([]ImageRequest, error)
	FindRequestsByUserID(userID uint) ([]ImageRequest, error)
	UpdateRequest(req *ImageRequest) error

	CreateAllowedImage(img *AllowedImage) error
	FindAllowedImages() ([]AllowedImage, error)
	FindAllowedImagesByProject(projectID uint) ([]AllowedImage, error)
	FindAllowedImagesForProject(projectID uint) ([]AllowedImage, error) // global + project-specific
	DeleteAllowedImage(id uint) error
	FindAllowedImageByNameTag(name, tag string) (*AllowedImage, error)
	ValidateImageForProject(name, tag string, projectID uint) (bool, error)
}
