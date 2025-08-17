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

type UserGroupService struct {
	Repos *repositories.Repos
}

func NewUserGroupService(repos *repositories.Repos) *UserGroupService {
	return &UserGroupService{
		Repos: repos,
	}
}

func (s *UserGroupService) AllocateGroupResource(gid uint, userName string) error {
	projects, err := s.Repos.Project.ListProjectsByGroup(gid)

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

func (s *UserGroupService) RemoveGroupResource(gid uint, userName string) error {
	projects, err := s.Repos.Project.ListProjectsByGroup(gid)

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

func (s *UserGroupService) CreateUserGroup(c *gin.Context, userGroup *models.UserGroup) (*models.UserGroup, error) {
	if err := s.Repos.UserGroup.CreateUserGroup(userGroup); err != nil {
		return nil, err
	}

	uesrName, err := s.Repos.User.GetUsernameByID(userGroup.UID)

	if err != nil {
		return nil, err
	}

	if err := s.AllocateGroupResource(userGroup.GID, uesrName); err != nil {
		return nil, err
	}

	utils.LogAuditWithConsole(c, "create", "user_group",
		fmt.Sprintf("u_id=%d,g_id=%d", userGroup.UID, userGroup.GID),
		nil, *userGroup, "", s.Repos.Audit)

	return userGroup, nil
}

func (s *UserGroupService) UpdateUserGroup(c *gin.Context, userGroup *models.UserGroup) (*models.UserGroup, error) {
	oldUserGroup, err := s.Repos.UserGroup.GetUserGroup(userGroup.UID, userGroup.GID)
	if err != nil {
		return nil, err
	}

	if err := s.Repos.UserGroup.UpdateUserGroup(userGroup); err != nil {
		return nil, err
	}

	if oldUserGroup.GID != userGroup.GID {
		uesrName, err := s.Repos.User.GetUsernameByID(userGroup.UID)

		if err != nil {
			return nil, err
		}

		if err := s.AllocateGroupResource(userGroup.GID, uesrName); err != nil {
			return nil, err
		}

		if err := s.RemoveGroupResource(oldUserGroup.GID, uesrName); err != nil {
			return nil, err
		}

	}
	utils.LogAuditWithConsole(c, "update", "user_group",
		fmt.Sprintf("u_id=%d,g_id=%d", userGroup.UID, userGroup.GID),
		oldUserGroup, *userGroup, "", s.Repos.Audit)

	return userGroup, nil
}

func (s *UserGroupService) DeleteUserGroup(c *gin.Context, uid, gid uint) error {
	oldUserGroup, err := s.Repos.UserGroup.GetUserGroup(uid, gid)
	if err != nil {
		return err
	}

	if err := s.Repos.UserGroup.DeleteUserGroup(uid, gid); err != nil {
		return err
	}

	uesrName, err := s.Repos.User.GetUsernameByID(uid)
	if err != nil {
		return err
	}

	if err := s.RemoveGroupResource(gid, uesrName); err != nil {
		return err
	}

	utils.LogAuditWithConsole(c, "delete", "user_group",
		fmt.Sprintf("u_id=%d,g_id=%d", uid, gid),
		oldUserGroup, nil, "", s.Repos.Audit)

	return nil
}

func (s *UserGroupService) GetUserGroup(uid, gid uint) (models.UserGroupView, error) {
	return s.Repos.UserGroup.GetUserGroup(uid, gid)
}

func (s *UserGroupService) GetUserGroupsByUID(uid uint) ([]models.UserGroupView, error) {
	return s.Repos.UserGroup.GetUserGroupsByUID(uid)
}

func (s *UserGroupService) GetUserGroupsByGID(gid uint) ([]models.UserGroupView, error) {
	return s.Repos.UserGroup.GetUserGroupsByGID(gid)
}

func (s *UserGroupService) GetFormattedUserGroupsByUID(uid uint) ([]dto.UserGroups, error) {
	records, err := s.Repos.UserGroup.GetUserGroupsByUID(uid)
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
