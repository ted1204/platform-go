package repositories

import (
	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/models"
)

type GroupRepo interface {
	GetAllGroups() ([]models.Group, error)
	GetGroupByID(id uint) (models.Group, error)
	CreateGroup(group *models.Group) error
	UpdateGroup(group *models.Group) error
	DeleteGroup(id uint) error
}

type DBGroupRepo struct{}

func NewGroupRepo() GroupRepo {
	return &DBGroupRepo{}
}
func (r *DBGroupRepo) GetAllGroups() ([]models.Group, error) {
	var groups []models.Group
	err := db.DB.Find(&groups).Error
	return groups, err
}

func (r *DBGroupRepo) GetGroupByID(id uint) (models.Group, error) {
	var group models.Group
	err := db.DB.First(&group, id).Error
	return group, err
}

func (r *DBGroupRepo) CreateGroup(group *models.Group) error {
	return db.DB.Create(group).Error
}

func (r *DBGroupRepo) UpdateGroup(group *models.Group) error {
	return db.DB.Save(group).Error
}

func (r *DBGroupRepo) DeleteGroup(id uint) error {
	return db.DB.Delete(&models.Group{}, id).Error
}
