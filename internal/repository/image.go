package repository

import (
	"github.com/linskybing/platform-go/internal/config/db"
	"github.com/linskybing/platform-go/internal/domain/image"
)

type ImageRepo interface {
	CreateRequest(req *image.ImageRequest) error
	ListRequests(status string) ([]image.ImageRequest, error)
	UpdateRequest(req *image.ImageRequest) error
	FindRequestByID(id uint) (*image.ImageRequest, error)
	CreateAllowed(img *image.AllowedImage) error
	ListAllowed() ([]image.AllowedImage, error)
	FindAllowedImagesByProject(projectID uint) ([]image.AllowedImage, error)
	FindAllowedImagesForProject(projectID uint) ([]image.AllowedImage, error)
	ValidateImageForProject(name, tag string, projectID uint) (bool, error)
	DeleteAllowedImage(id uint) error
}

type DBImageRepo struct{}

func (r *DBImageRepo) CreateRequest(req *image.ImageRequest) error {
	return db.DB.Create(req).Error
}

func (r *DBImageRepo) ListRequests(status string) ([]image.ImageRequest, error) {
	var reqs []image.ImageRequest
	q := db.DB.Model(&image.ImageRequest{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("created_at desc").Find(&reqs).Error
	return reqs, err
}

func (r *DBImageRepo) FindRequestByID(id uint) (*image.ImageRequest, error) {
	var req image.ImageRequest
	if err := db.DB.First(&req, id).Error; err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *DBImageRepo) UpdateRequest(req *image.ImageRequest) error {
	return db.DB.Save(req).Error
}

func (r *DBImageRepo) CreateAllowed(img *image.AllowedImage) error {
	return db.DB.Create(img).Error
}

func (r *DBImageRepo) ListAllowed() ([]image.AllowedImage, error) {
	var imgs []image.AllowedImage
	err := db.DB.Order("created_at desc").Find(&imgs).Error
	return imgs, err
}

func (r *DBImageRepo) FindAllowedImagesByProject(projectID uint) ([]image.AllowedImage, error) {
	var imgs []image.AllowedImage
	err := db.DB.Where("project_id = ?", projectID).Order("created_at desc").Find(&imgs).Error
	return imgs, err
}

// FindAllowedImagesForProject returns global images + project-specific images
func (r *DBImageRepo) FindAllowedImagesForProject(projectID uint) ([]image.AllowedImage, error) {
	var imgs []image.AllowedImage
	err := db.DB.Where("is_global = ? OR project_id = ?", true, projectID).
		Order("is_global desc, created_at desc").Find(&imgs).Error
	return imgs, err
}

// ValidateImageForProject checks if an image is allowed for a project
func (r *DBImageRepo) ValidateImageForProject(name, tag string, projectID uint) (bool, error) {
	var count int64
	err := db.DB.Model(&image.AllowedImage{}).
		Where("name = ? AND tag = ? AND (is_global = ? OR project_id = ?)",
			name, tag, true, projectID).
		Count(&count).Error
	return count > 0, err
}

func (r *DBImageRepo) DeleteAllowedImage(id uint) error {
	return db.DB.Delete(&image.AllowedImage{}, id).Error
}
