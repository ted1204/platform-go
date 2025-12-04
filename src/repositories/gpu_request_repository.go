package repositories

import (
	"github.com/linskybing/platform-go/src/db"
	"github.com/linskybing/platform-go/src/models"
)

type GPURequestRepo interface {
	Create(req *models.GPURequest) error
	Update(req *models.GPURequest) error
	GetByID(id uint) (models.GPURequest, error)
	ListByProjectID(projectID uint) ([]models.GPURequest, error)
	ListPending() ([]models.GPURequest, error)
}

type DBGPURequestRepo struct{}

func (r *DBGPURequestRepo) Create(req *models.GPURequest) error {
	return db.DB.Create(req).Error
}

func (r *DBGPURequestRepo) Update(req *models.GPURequest) error {
	return db.DB.Save(req).Error
}

func (r *DBGPURequestRepo) GetByID(id uint) (models.GPURequest, error) {
	var req models.GPURequest
	err := db.DB.First(&req, id).Error
	return req, err
}

func (r *DBGPURequestRepo) ListByProjectID(projectID uint) ([]models.GPURequest, error) {
	var reqs []models.GPURequest
	err := db.DB.Where("project_id = ?", projectID).Find(&reqs).Error
	return reqs, err
}

func (r *DBGPURequestRepo) ListPending() ([]models.GPURequest, error) {
	var reqs []models.GPURequest
	err := db.DB.Where("status = ?", models.GPURequestStatusPending).Find(&reqs).Error
	return reqs, err
}
