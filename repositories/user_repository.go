package repositories

import (
	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/models"
)

type UserRepo interface {
	GetAllUsers() ([]models.UserWithSuperAdmin, error)
	ListUsersPaging(page, limit int) ([]models.UserWithSuperAdmin, error)
	GetUserByID(id uint) (models.UserWithSuperAdmin, error)
	GetUsernameByID(id uint) (string, error)
	GetUserRawByID(id uint) (models.User, error)
	SaveUser(user *models.User) error
	DeleteUser(id uint) error
}

type DBUserRepo struct{}

func (r *DBUserRepo) GetAllUsers() ([]models.UserWithSuperAdmin, error) {
	var users []models.UserWithSuperAdmin
	if err := db.DB.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *DBUserRepo) ListUsersPaging(page, limit int) ([]models.UserWithSuperAdmin, error) {
	var users []models.UserWithSuperAdmin

	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = 10
	}

	offset := (page - 1) * limit

	if err := db.DB.Offset(int(offset)).Limit(int(limit)).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *DBUserRepo) GetUserByID(id uint) (models.UserWithSuperAdmin, error) {
	var user models.UserWithSuperAdmin
	if err := db.DB.First(&user, id).Error; err != nil {
		return models.UserWithSuperAdmin{}, err
	}
	return user, nil
}

func (r *DBUserRepo) GetUsernameByID(id uint) (string, error) {
	var user string
	err := db.DB.Model(&models.User{}).Select("username").Where("u_id = ?", id).First(&user).Error
	if err != nil {
		return "", err
	}
	return user, nil
}

func (r *DBUserRepo) GetUserRawByID(id uint) (models.User, error) {
	var user models.User
	if err := db.DB.First(&user, id).Error; err != nil {
		return models.User{}, err
	}
	return user, nil
}

func (r *DBUserRepo) SaveUser(user *models.User) error {
	return db.DB.Save(user).Error
}

func (r *DBUserRepo) DeleteUser(id uint) error {
	return db.DB.Delete(&models.User{}, id).Error
}
