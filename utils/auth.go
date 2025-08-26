package utils

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/config"
	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/models"
	"github.com/linskybing/platform-go/repositories"
	"github.com/linskybing/platform-go/types"
	"gorm.io/gorm"
)

func IsSuperAdmin(uid uint, repos repositories.ViewRepo) (bool, error) {
	return repos.IsSuperAdmin(uid)
}

var GetUserIDFromContext = func(c *gin.Context) (uint, error) {
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

func CheckGroupPermission(UID uint, GID uint, repos repositories.ViewRepo) (bool, error) {
	// Check if the user is a group manager (admin role in the group)
	isManager, err := HasGroupRole(UID, GID, config.GroupUpdateRoles)
	if err != nil {
		return false, err
	}
	if isManager {
		return true, nil
	}

	// If not group admin, check if the user is a super admin
	isSuper, err := repos.IsSuperAdmin(UID)
	if err != nil {
		return false, err
	}
	if isSuper {
		return true, nil
	}

	return false, errors.New("permission denied")
}

func CheckGroupAdminPermission(UID uint, GID uint, repos repositories.ViewRepo) (bool, error) {
	// Check if the user is a group manager (admin role in the group)
	isManager, err := HasGroupRole(UID, GID, config.GroupAdminRoles)
	if err != nil {
		return false, err
	}
	if isManager {
		return true, nil
	}

	// If not group admin, check if the user is a super admin
	isSuper, err := repos.IsSuperAdmin(UID)
	if err != nil {
		return false, err
	}
	if isSuper {
		return true, nil
	}

	return false, errors.New("permission denied")
}
