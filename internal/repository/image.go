package repository

import (
	"strings"

	"github.com/linskybing/platform-go/internal/config"
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
	FindAllowedByID(id uint) (*image.AllowedImage, error)
	ValidateImageForProject(name, tag string, projectID uint) (bool, error)
	FindAllowedImage(name, tag string, projectID uint) (*image.AllowedImage, error)
	DeleteAllowedImage(id uint) error
	UpdateImagePulledStatus(name, tag string, isPulled bool) error
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
	// If the provided image name already contains the Harbor private prefix,
	// normalize by removing the prefix and mark the record as pulled.
	if strings.HasPrefix(img.Name, config.HarborPrivatePrefix) {
		img.Name = strings.TrimPrefix(img.Name, config.HarborPrivatePrefix)
		img.IsPulled = true
	}

	// If there already exists an allowed image with the same name+tag that
	// has been pulled, mark the new record as pulled as well so create/update
	// behave consistently across duplicate entries.
	var existing image.AllowedImage
	if err := db.DB.Where("name = ? AND tag = ? AND is_pulled = ?", img.Name, img.Tag, true).First(&existing).Error; err == nil {
		img.IsPulled = true
	}

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

func (r *DBImageRepo) FindAllowedByID(id uint) (*image.AllowedImage, error) {
	var img image.AllowedImage
	if err := db.DB.First(&img, id).Error; err != nil {
		return nil, err
	}
	return &img, nil
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

// FindAllowedImage retrieves an allowed image by name, tag, and project
func (r *DBImageRepo) FindAllowedImage(name, tag string, projectID uint) (*image.AllowedImage, error) {
	var img image.AllowedImage
	err := db.DB.Where("name = ? AND tag = ? AND (is_global = ? OR project_id = ?)",
		name, tag, true, projectID).
		First(&img).Error
	if err != nil {
		return nil, err
	}
	return &img, nil
}

func (r *DBImageRepo) DeleteAllowedImage(id uint) error {
	return db.DB.Delete(&image.AllowedImage{}, id).Error
}

// UpdateImagePulledStatus marks an image as pulled after successful pull job
func (r *DBImageRepo) UpdateImagePulledStatus(name, tag string, isPulled bool) error {
	return db.DB.Model(&image.AllowedImage{}).
		Where("name = ? AND tag = ?", name, tag).
		Update("is_pulled", isPulled).Error
}
