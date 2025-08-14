package services

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/config"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/models"
	"github.com/linskybing/platform-go/repositories"
	"github.com/linskybing/platform-go/utils"
)

func AllocateGroupResource(gid uint, userName string) error {
	projects, err := GetProjectsByGroupId(gid)

	if err != nil {
		return err
	}

	for _, project := range projects {
		ns := utils.FormatNamespaceName(project.PID, userName)
		if err := utils.CreateNamespace(ns); err != nil {
			return err
		}
		if err := utils.CreatePVC(ns, config.DefaultStorageName, config.DefaultStorageClassName, config.DefaultStorageSize); err != nil {
			return err
		}
	}

	return nil
}

func RemoveGroupResource(gid uint, userName string) error {
	projects, err := GetProjectsByGroupId(gid)

	if err != nil {
		return err
	}

	for _, project := range projects {
		ns := utils.FormatNamespaceName(project.PID, userName)
		if err := utils.DeleteNamespace(ns); err != nil {
			return err
		}
	}

	return nil
}

func CreateUserGroup(c *gin.Context, userGroup *models.UserGroup) (*models.UserGroup, error) {
	if err := repositories.CreateUserGroup(userGroup); err != nil {
		return nil, err
	}

	uesrName, err := repositories.GetUsernameByID(userGroup.UID)

	if err != nil {
		return nil, err
	}

	if err := AllocateGroupResource(userGroup.GID, uesrName); err != nil {
		return nil, err
	}

	utils.LogAuditWithConsole(c, "create", "user_group",
		fmt.Sprintf("u_id=%d,g_id=%d", userGroup.UID, userGroup.GID),
		nil, *userGroup, "")

	return userGroup, nil
}

func UpdateUserGroup(c *gin.Context, userGroup *models.UserGroup) (*models.UserGroup, error) {
	oldUserGroup, err := repositories.GetUserGroup(userGroup.UID, userGroup.GID)
	if err != nil {
		return nil, err
	}

	if err := repositories.UpdateUserGroup(userGroup); err != nil {
		return nil, err
	}

	if oldUserGroup.GID != userGroup.GID {
		uesrName, err := repositories.GetUsernameByID(userGroup.UID)

		if err != nil {
			return nil, err
		}

		if err := AllocateGroupResource(userGroup.GID, uesrName); err != nil {
			return nil, err
		}

		if err := RemoveGroupResource(oldUserGroup.GID, uesrName); err != nil {
			return nil, err
		}

	}
	utils.LogAuditWithConsole(c, "update", "user_group",
		fmt.Sprintf("u_id=%d,g_id=%d", userGroup.UID, userGroup.GID),
		oldUserGroup, *userGroup, "")

	return userGroup, nil
}

func DeleteUserGroup(c *gin.Context, uid, gid uint) error {
	oldUserGroup, err := repositories.GetUserGroup(uid, gid)
	if err != nil {
		return err
	}

	if err := repositories.DeleteUserGroup(uid, gid); err != nil {
		return err
	}

	uesrName, err := repositories.GetUsernameByID(uid)
	if err != nil {
		return err
	}

	if err := RemoveGroupResource(gid, uesrName); err != nil {
		return err
	}

	utils.LogAuditWithConsole(c, "delete", "user_group",
		fmt.Sprintf("u_id=%d,g_id=%d", uid, gid),
		oldUserGroup, nil, "")

	return nil
}

func GetUserGroup(uid, gid uint) (models.UserGroupView, error) {
	return repositories.GetUserGroup(uid, gid)
}

func GetUserGroupsByUID(uid uint) ([]models.UserGroupView, error) {
	return repositories.GetUserGroupsByUID(uid)
}

func GetUserGroupsByGID(gid uint) ([]models.UserGroupView, error) {
	return repositories.GetUserGroupsByGID(gid)
}

func GetFormattedUserGroupsByUID(uid uint) ([]dto.UserGroups, error) {
	records, err := repositories.GetUserGroupsByUID(uid)
	if err != nil {
		return nil, err
	}

	userMap := make(map[uint]*dto.UserGroups)
	for _, r := range records {
		if _, exists := userMap[r.UID]; !exists {
			userMap[r.UID] = &dto.UserGroups{
				UID:      r.UID,
				Username: r.Username,
				Groups:   []dto.GroupInfo{},
			}
		}
		userMap[r.UID].Groups = append(userMap[r.UID].Groups, dto.GroupInfo{
			GID:       r.GID,
			GroupName: r.GroupName,
			Role:      r.Role,
		})
	}

	result := make([]dto.UserGroups, 0, len(userMap))
	for _, v := range userMap {
		result = append(result, *v)
	}
	return result, nil
}
