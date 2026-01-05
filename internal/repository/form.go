package repository

import (
	"github.com/linskybing/platform-go/internal/config/db"
	"github.com/linskybing/platform-go/internal/domain/form"
)

type FormRepository struct{}

func NewFormRepository() *FormRepository {
	return &FormRepository{}
}

func (r *FormRepository) Create(form *form.Form) error {
	return db.DB.Create(form).Error
}

func (r *FormRepository) CreateMessage(msg *form.FormMessage) error {
	return db.DB.Create(msg).Error
}

func (r *FormRepository) FindAll() ([]form.Form, error) {
	var forms []form.Form
	err := db.DB.Preload("User").Preload("Project").Order("created_at desc").Find(&forms).Error
	return forms, err
}

func (r *FormRepository) FindByUserID(userID uint) ([]form.Form, error) {
	var forms []form.Form
	err := db.DB.Where("user_id = ?", userID).Preload("User").Preload("Project").Order("created_at desc").Find(&forms).Error
	return forms, err
}

func (r *FormRepository) FindByID(id uint) (*form.Form, error) {
	var form form.Form
	err := db.DB.Preload("User").Preload("Project").Preload("Messages").First(&form, id).Error
	return &form, err
}

func (r *FormRepository) Update(form *form.Form) error {
	return db.DB.Save(form).Error
}

func (r *FormRepository) ListMessages(formID uint) ([]form.FormMessage, error) {
	var msgs []form.FormMessage
	err := db.DB.Where("form_id = ?", formID).Order("created_at asc").Find(&msgs).Error
	return msgs, err
}
