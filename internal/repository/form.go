package repository

import (
	"github.com/linskybing/platform-go/internal/domain/form"
	"gorm.io/gorm"
)

type FormRepo interface {
	Create(form *form.Form) error
	CreateMessage(msg *form.FormMessage) error
	FindAll() ([]form.Form, error)
	FindByUserID(userID uint) ([]form.Form, error)
	FindByID(id uint) (*form.Form, error)
	Update(form *form.Form) error
	ListMessages(formID uint) ([]form.FormMessage, error)
	WithTx(tx *gorm.DB) FormRepo
}

type DBFormRepo struct {
	db *gorm.DB
}

func NewFormRepo(db *gorm.DB) *DBFormRepo {
	return &DBFormRepo{
		db: db,
	}
}

func (r *DBFormRepo) Create(form *form.Form) error {
	return r.db.Create(form).Error
}

func (r *DBFormRepo) CreateMessage(msg *form.FormMessage) error {
	return r.db.Create(msg).Error
}

func (r *DBFormRepo) FindAll() ([]form.Form, error) {
	var forms []form.Form
	err := r.db.Preload("User").Preload("Project").Order("created_at desc").Find(&forms).Error
	return forms, err
}

func (r *DBFormRepo) FindByUserID(userID uint) ([]form.Form, error) {
	var forms []form.Form
	err := r.db.Where("user_id = ?", userID).Preload("User").Preload("Project").Order("created_at desc").Find(&forms).Error
	return forms, err
}

func (r *DBFormRepo) FindByID(id uint) (*form.Form, error) {
	var form form.Form
	err := r.db.Preload("User").Preload("Project").Preload("Messages").First(&form, id).Error
	return &form, err
}

func (r *DBFormRepo) Update(form *form.Form) error {
	return r.db.Save(form).Error
}

func (r *DBFormRepo) ListMessages(formID uint) ([]form.FormMessage, error) {
	var msgs []form.FormMessage
	err := r.db.Where("form_id = ?", formID).Order("created_at asc").Find(&msgs).Error
	return msgs, err
}

func (r *DBFormRepo) WithTx(tx *gorm.DB) FormRepo {
	if tx == nil {
		return r
	}
	return &DBFormRepo{
		db: tx,
	}
}
