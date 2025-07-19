package repositories

import (
	"errors"

	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/models"
	"gorm.io/gorm"
)

func CreateResource(resource *models.Resource) error {
	return db.DB.Create(resource).Error
}

func GetResourceByID(rid uint) (*models.Resource, error) {
	var resource models.Resource
	err := db.DB.First(&resource, "r_id = ?", rid).Error
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

func UpdateResource(resource *models.Resource) error {
	if resource.RID == 0 {
		return errors.New("resource RID is required")
	}
	return db.DB.Save(resource).Error
}

func DeleteResource(rid uint) error {
	return db.DB.Delete(&models.Resource{}, "r_id = ?", rid).Error
}

func ListResourcesByProjectID(pid uint) ([]models.Resource, error) {
	var resources []models.Resource
	err := db.DB.
		Joins("JOIN config_files cf ON cf.cf_id = resources.cf_id").
		Where("cf.project_id = ?", pid).
		Find(&resources).Error
	return resources, err
}

func ListResourcesByConfigFileID(cfID uint) ([]models.Resource, error) {
	var resources []models.Resource
	err := db.DB.
		Where("cf_id = ?", cfID).
		Find(&resources).Error
	return resources, err
}

func GetResourceByConfigFileIDAndName(cfID uint, name string) (*models.Resource, error) {
	var resource models.Resource
	err := db.DB.
		Where("cf_id = ? AND name = ?", cfID, name).
		First(&resource).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &resource, nil
}
