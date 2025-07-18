package repositories

import (
	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/models"
)

func GetProjectByID(id uint) (models.Project, error) {
	var project models.Project
	err := db.DB.First(&project, id).Error
	return project, err
}

func GetGroupIDByProjectID(pID uint) (uint, error) {
	var gID uint
	err := db.DB.Model(&models.Project{}).Select("g_id").Where("p_id = ?", pID).Scan(&gID).Error
	if err != nil {
		return 0, err
	}
	return gID, nil
}

func CreateProject(p *models.Project) error {
	return db.DB.Create(p).Error
}

func UpdateProject(p *models.Project) error {
	return db.DB.Save(p).Error
}

func DeleteProject(id uint) error {
	return db.DB.Delete(&models.Project{}, id).Error
}

func ListProjects() ([]models.Project, error) {
	var projects []models.Project
	err := db.DB.Find(&projects).Error
	return projects, err
}
