package repositories

import (
	"errors"

	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/models"
)

type ConfigFileRepo interface {
	CreateConfigFile(cf *models.ConfigFile) error
	GetConfigFileByID(id uint) (*models.ConfigFile, error)
	UpdateConfigFile(cf *models.ConfigFile) error
	DeleteConfigFile(id uint) error
	ListConfigFiles() ([]models.ConfigFile, error)
	GetConfigFilesByProjectID(projectID uint) ([]models.ConfigFile, error)
}

type DBConfigFileRepo struct{}

func (r *DBConfigFileRepo) CreateConfigFile(cf *models.ConfigFile) error {
	return db.DB.Create(cf).Error
}

func (r *DBConfigFileRepo) GetConfigFileByID(id uint) (*models.ConfigFile, error) {
	var cf models.ConfigFile
	if err := db.DB.First(&cf, id).Error; err != nil {
		return nil, err
	}
	return &cf, nil
}

func (r *DBConfigFileRepo) UpdateConfigFile(cf *models.ConfigFile) error {
	if cf.CFID == 0 {
		return errors.New("missing ConfigFile ID")
	}
	return db.DB.Save(cf).Error
}

func (r *DBConfigFileRepo) DeleteConfigFile(id uint) error {
	return db.DB.Delete(&models.ConfigFile{}, id).Error
}

func (r *DBConfigFileRepo) ListConfigFiles() ([]models.ConfigFile, error) {
	var list []models.ConfigFile
	if err := db.DB.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *DBConfigFileRepo) GetConfigFilesByProjectID(projectID uint) ([]models.ConfigFile, error) {
	var files []models.ConfigFile
	if err := db.DB.Where("project_id = ?", projectID).Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}
