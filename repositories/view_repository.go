package repositories

import (
	"errors"

	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/models"
	"gorm.io/gorm"
)

type ViewRepo interface {
	GetAllProjectGroupViews() ([]models.ProjectGroupView, error)
	GetProjectResourcesByGroupID(groupID uint) ([]models.ProjectResourceView, error)
	GetGroupResourcesByGroupID(groupID uint) ([]models.GroupResourceView, error)
	GetGroupIDByResourceID(rID uint) (uint, error)
	GetGroupIDByConfigFileID(cfID uint) (uint, error)
	IsSuperAdmin(uid uint) (bool, error)
	ListUsersByProjectID(projectID uint) ([]models.ProjectUserView, error)
	ListProjectsByUserID(userID uint) ([]models.ProjectUserView, error)
}

type DBViewRepo struct{}

func (r *DBViewRepo) GetAllProjectGroupViews() ([]models.ProjectGroupView, error) {
	var results []models.ProjectGroupView
	err := db.DB.Find(&results).Error
	return results, err
}

func (r *DBViewRepo) GetProjectResourcesByGroupID(groupID uint) ([]models.ProjectResourceView, error) {
	var results []models.ProjectResourceView
	err := db.DB.Where("g_id = ?", groupID).Find(&results).Error
	return results, err
}

func (r *DBViewRepo) GetGroupResourcesByGroupID(groupID uint) ([]models.GroupResourceView, error) {
	var results []models.GroupResourceView
	err := db.DB.Where("g_id = ?", groupID).Find(&results).Error
	return results, err
}

func (r *DBViewRepo) GetGroupIDByResourceID(rID uint) (uint, error) {
	type result struct {
		GID uint `gorm:"column:g_id"`
	}

	var res result

	err := db.DB.Table("resources r").
		Select("p.g_id").
		Joins("JOIN config_files cf ON cf.cf_id = r.cf_id").
		Joins("JOIN projects p ON cf.project_id = p.p_id").
		Where("r.r_id = ?", rID).
		Take(&res).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, gorm.ErrRecordNotFound
	}
	if err != nil {
		return 0, err
	}

	return res.GID, nil
}

func (r *DBViewRepo) GetGroupIDByConfigFileID(cfID uint) (uint, error) {
	type result struct {
		GID uint `gorm:"column:g_id"`
	}

	var res result

	err := db.DB.Table("config_files cf").
		Select("p.g_id").
		Joins("JOIN projects p ON cf.project_id = p.p_id").
		Where("cf.cf_id = ?", cfID).
		Take(&res).Error

	if err != nil {
		return 0, err
	}

	return res.GID, nil
}

func (r *DBViewRepo) IsSuperAdmin(uid uint) (bool, error) {
	var view models.UserGroupView
	err := db.DB.
		Where("u_id = ? AND group_name = ? AND role = ?", uid, "super", "admin").
		First(&view).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *DBViewRepo) ListUsersByProjectID(projectID uint) ([]models.ProjectUserView, error) {
	var users []models.ProjectUserView
	err := db.DB.Where("p_id = ?", projectID).Find(&users).Error
	return users, err
}

func (r *DBViewRepo) ListProjectsByUserID(userID uint) ([]models.ProjectUserView, error) {
	var projects []models.ProjectUserView
	if err := db.DB.Where("u_id = ?", userID).Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}
