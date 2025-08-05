package repositories

import (
	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/models"
)

func GetAllUsers() ([]models.UserWithSuperAdmin, error) {
	var users []models.UserWithSuperAdmin
	if err := db.DB.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func GetUserByID(id uint) (models.UserWithSuperAdmin, error) {
	var user models.UserWithSuperAdmin
	if err := db.DB.First(&user, id).Error; err != nil {
		return models.UserWithSuperAdmin{}, err
	}
	return user, nil
}

func GetUsernameByID(id uint) (string, error) {
	var user string
	err := db.DB.Model(&models.User{}).Select("username").Where("u_id = ?", id).First(&user).Error
	if err != nil {
		return "", err
	}
	return user, nil
}

func GetUserRawByID(id uint) (models.User, error) {
	var user models.User
	if err := db.DB.First(&user, id).Error; err != nil {
		return models.User{}, err
	}
	return user, nil
}

func SaveUser(user *models.User) error {
	return db.DB.Save(user).Error
}

func DeleteUser(id uint) error {
	return db.DB.Delete(&models.User{}, id).Error
}
