package repositories

import (
	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/models"
)

type ProjectRepo interface {
	GetProjectByID(id uint) (models.Project, error)
	GetGroupIDByProjectID(pID uint) (uint, error)
	CreateProject(p *models.Project) error
	UpdateProject(p *models.Project) error
	DeleteProject(id uint) error
	ListProjects() ([]models.Project, error)
	ListProjectsByGroup(id uint) ([]models.Project, error)
}

type DBProjectRepo struct{}

func (r *DBProjectRepo) GetProjectByID(id uint) (models.Project, error) {
	var project models.Project
	err := db.DB.First(&project, id).Error
	return project, err
}

func (r *DBProjectRepo) GetGroupIDByProjectID(pID uint) (uint, error) {
	var gID uint
	err := db.DB.Model(&models.Project{}).Select("g_id").Where("p_id = ?", pID).Scan(&gID).Error
	if err != nil {
		return 0, err
	}
	return gID, nil
}

func (r *DBProjectRepo) CreateProject(p *models.Project) error {
	return db.DB.Create(p).Error
}

func (r *DBProjectRepo) UpdateProject(p *models.Project) error {
	return db.DB.Save(p).Error
}

func (r *DBProjectRepo) DeleteProject(id uint) error {
	return db.DB.Delete(&models.Project{}, id).Error
}

func (r *DBProjectRepo) ListProjects() ([]models.Project, error) {
	var projects []models.Project
	err := db.DB.Find(&projects).Error
	return projects, err
}

func (r *DBProjectRepo) ListProjectsByGroup(id uint) ([]models.Project, error) {
	var projects []models.Project
	if err := db.DB.Where("g_id = ?", id).Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}
