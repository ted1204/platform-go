package repositories

import (
	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/models"
)

type UserGroupRepo interface {
	CreateUserGroup(userGroup *models.UserGroup) error
	UpdateUserGroup(userGroup *models.UserGroup) error
	DeleteUserGroup(uid, gid uint) error
	GetUserGroupsByUID(uid uint) ([]models.UserGroupView, error)
	GetUserGroupsByGID(gid uint) ([]models.UserGroupView, error)
	GetUserGroup(uid, gid uint) (models.UserGroupView, error)
}

type DBUserGroupRepo struct{}

func (r *DBUserGroupRepo) CreateUserGroup(userGroup *models.UserGroup) error {
	return db.DB.Create(userGroup).Error
}

func (r *DBUserGroupRepo) UpdateUserGroup(userGroup *models.UserGroup) error {
	return db.DB.Save(userGroup).Error
}

func (r *DBUserGroupRepo) DeleteUserGroup(uid, gid uint) error {
	return db.DB.Where("u_id = ? AND g_id = ?", uid, gid).Delete(&models.UserGroup{}).Error
}

func (r *DBUserGroupRepo) GetUserGroupsByUID(uid uint) ([]models.UserGroupView, error) {
	var userGroups []models.UserGroupView
	err := db.DB.
		Where("u_id = ?", uid).
		Find(&userGroups).Error
	return userGroups, err
}

func (r *DBUserGroupRepo) GetUserGroupsByGID(gid uint) ([]models.UserGroupView, error) {
	var userGroups []models.UserGroupView
	err := db.DB.
		Where("g_id = ?", gid).
		Find(&userGroups).Error
	return userGroups, err
}

func (r *DBUserGroupRepo) GetUserGroup(uid, gid uint) (models.UserGroupView, error) {
	var userGroup models.UserGroupView
	err := db.DB.First(&userGroup, "u_id = ? AND g_id = ?", uid, gid).Error
	return userGroup, err
}
