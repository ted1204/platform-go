package repositories

import (
	"github.com/linskybing/platform-go/src/db"
	"github.com/linskybing/platform-go/src/models"
)

type FormRepository struct{}

func NewFormRepository() *FormRepository {
	return &FormRepository{}
}

func (r *FormRepository) Create(form *models.Form) error {
	return db.DB.Create(form).Error
}

func (r *FormRepository) FindAll() ([]models.Form, error) {
	var forms []models.Form
	err := db.DB.Preload("User").Preload("Project").Order("created_at desc").Find(&forms).Error
	return forms, err
}

func (r *FormRepository) FindByUserID(userID uint) ([]models.Form, error) {
	var forms []models.Form
	err := db.DB.Where("user_id = ?", userID).Preload("User").Preload("Project").Order("created_at desc").Find(&forms).Error
	return forms, err
}

func (r *FormRepository) FindByID(id uint) (*models.Form, error) {
	var form models.Form
	err := db.DB.Preload("User").Preload("Project").First(&form, id).Error
	return &form, err
}

func (r *FormRepository) Update(form *models.Form) error {
	return db.DB.Save(form).Error
}
