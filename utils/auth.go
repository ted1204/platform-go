package utils

import (
	"errors"

	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/models"
	"gorm.io/gorm"
)

func IsSuperAdmin(uid uint) (bool, error) {
	var view models.UserGroupView
	err := db.DB.
		Where("u_id = ? AND group_name = ? AND role = ?", uid, "super", "admin").
		First(&view).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
