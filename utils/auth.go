package utils

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/config"
	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/models"
	"github.com/linskybing/platform-go/types"
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

func GetUserIDFromContext(c *gin.Context) (uint, error) {
	claimsVal, exists := c.Get("claims")
	if !exists {
		return 0, errors.New("user claims not found in context")
	}

	claims, ok := claimsVal.(*types.Claims)
	if !ok {
		return 0, errors.New("invalid user claims type")
	}

	return claims.UserID, nil
}

func HasGroupRole(userID uint, gid uint, roles []string) (bool, error) {
	var view models.UserGroupView
	err := db.DB.
		Where("u_id = ? AND g_id = ? AND role IN ?", userID, gid, roles).
		First(&view).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func CheckProjectCreatePermission(UID uint, GIDs []uint) (bool, error) {
	allManager := true
	for _, gid := range GIDs {
		isManager, _ := HasGroupRole(UID, gid, config.GroupAdminRoles)
		if !isManager {
			allManager = false
			break
		}
	}

	if allManager {
		return true, nil
	}

	isSuper, err := IsSuperAdmin(UID)
	if err != nil {
		return false, err
	}
	if isSuper {
		return true, nil
	}
	return false, errors.New("permission denied")
}
