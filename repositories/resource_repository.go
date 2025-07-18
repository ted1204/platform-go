package repositories

import (
	"errors"

	"github.com/linskybing/platform-go/models"
	"gorm.io/gorm"
)

type ResourceRepository struct {
	DB *gorm.DB
}

func (r *ResourceRepository) CreateResource(resource *models.Resource) error {
	return r.DB.Create(resource).Error
}

func (r *ResourceRepository) GetResourceByID(rid uint) (*models.Resource, error) {
	var resource models.Resource
	err := r.DB.First(&resource, "r_id = ?", rid).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &resource, nil
}

func (r *ResourceRepository) UpdateResource(resource *models.Resource) error {
	if resource.RID == 0 {
		return errors.New("resource RID is required")
	}
	return r.DB.Save(resource).Error
}

func (r *ResourceRepository) DeleteResource(rid uint) error {
	return r.DB.Delete(&models.Resource{}, "r_id = ?", rid).Error
}

func (r *ResourceRepository) ListResourcesByProjectID(pid uint) ([]models.Resource, error) {
	var resources []models.Resource
	err := r.DB.Where("p_id = ?", pid).Find(&resources).Error
	return resources, err
}
