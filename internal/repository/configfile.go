package repository

import (
	"errors"

	"github.com/linskybing/platform-go/internal/domain/configfile"
	"gorm.io/gorm"
)

type ConfigFileRepo interface {
	CreateConfigFile(cf *configfile.ConfigFile) error
	GetConfigFileByID(id uint) (*configfile.ConfigFile, error)
	UpdateConfigFile(cf *configfile.ConfigFile) error
	DeleteConfigFile(id uint) error
	ListConfigFiles() ([]configfile.ConfigFile, error)
	GetConfigFilesByProjectID(projectID uint) ([]configfile.ConfigFile, error)
	GetGroupIDByConfigFileID(cfID uint) (uint, error)
	WithTx(tx *gorm.DB) ConfigFileRepo
}

type DBConfigFileRepo struct {
	db *gorm.DB
}

func NewConfigFileRepo(db *gorm.DB) *DBConfigFileRepo {
	return &DBConfigFileRepo{
		db: db,
	}
}

func (r *DBConfigFileRepo) CreateConfigFile(cf *configfile.ConfigFile) error {
	return r.db.Create(cf).Error
}

func (r *DBConfigFileRepo) GetConfigFileByID(id uint) (*configfile.ConfigFile, error) {
	var cf configfile.ConfigFile
	if err := r.db.First(&cf, id).Error; err != nil {
		return nil, err
	}
	return &cf, nil
}

func (r *DBConfigFileRepo) UpdateConfigFile(cf *configfile.ConfigFile) error {
	if cf.CFID == 0 {
		return errors.New("missing ConfigFile ID")
	}
	return r.db.Save(cf).Error
}

func (r *DBConfigFileRepo) DeleteConfigFile(id uint) error {
	return r.db.Delete(&configfile.ConfigFile{}, id).Error
}

func (r *DBConfigFileRepo) ListConfigFiles() ([]configfile.ConfigFile, error) {
	var list []configfile.ConfigFile
	if err := r.db.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *DBConfigFileRepo) GetConfigFilesByProjectID(projectID uint) ([]configfile.ConfigFile, error) {
	var files []configfile.ConfigFile
	if err := r.db.Where("project_id = ?", projectID).Find(&files).Error; err != nil {
		return nil, err
	}
	return files, nil
}

func (r *DBConfigFileRepo) GetGroupIDByConfigFileID(cfID uint) (uint, error) {
	var gID uint
	err := r.db.Table("config_files cf").
		Select("p.g_id").
		Joins("JOIN project_list p ON cf.project_id = p.p_id").
		Where("cf.cf_id = ?", cfID).
		Scan(&gID).Error

	if err != nil {
		return 0, err
	}
	return gID, nil
}

func (r *DBConfigFileRepo) WithTx(tx *gorm.DB) ConfigFileRepo {
	if tx == nil {
		return r
	}
	return &DBConfigFileRepo{
		db: tx,
	}
}
