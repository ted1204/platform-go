package services

import (
	"errors"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/models"
	"github.com/linskybing/platform-go/src/repositories"
	"github.com/linskybing/platform-go/src/utils"
)

var ErrReservedUser = errors.New("cannot modify reserved user & group 'admin & super'")

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
		// if err := utils.CreatePVC(ns, config.DefaultStorageName, config.DefaultStorageClassName, config.DefaultStorageSize); err != nil {
		// 	return err
		// }
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

	go utils.LogAuditWithConsole(c, "create", "user_group",
		fmt.Sprintf("u_id=%d,g_id=%d", userGroup.UID, userGroup.GID),
		nil, *userGroup, "", s.Repos.Audit)

	return userGroup, nil
}

func (s *UserGroupService) UpdateUserGroup(c *gin.Context, userGroup *models.UserGroup, existing models.UserGroupView) (*models.UserGroup, error) {
	if err := s.Repos.UserGroup.UpdateUserGroup(userGroup); err != nil {
		return nil, err
	}

	go utils.LogAuditWithConsole(c, "update", "user_group",
		fmt.Sprintf("u_id=%d,g_id=%d", userGroup.UID, userGroup.GID),
		existing, *userGroup, "", s.Repos.Audit)

	return userGroup, nil
}

func (s *UserGroupService) DeleteUserGroup(c *gin.Context, uid, gid uint) error {
	oldUserGroup, err := s.Repos.UserGroup.GetUserGroup(uid, gid)
	if err != nil {
		return err
	}

	if uid == 1 && gid == 1 || uid == 1 && oldUserGroup.GroupName == "super" {
		return ErrReservedUser
	}
	log.Printf("sddf")
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

	go utils.LogAuditWithConsole(c, "delete", "user_group",
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

func (s *UserGroupService) FormatByUID(records []models.UserGroupView) map[uint]dto.UserGroups {
	result := make(map[uint]dto.UserGroups)

	for _, r := range records {
		if g, exists := result[r.UID]; exists {
			g.Groups = append(g.Groups, dto.GroupInfo{
				GID:       r.GID,
				GroupName: r.GroupName,
				Role:      r.Role,
			})
			result[r.UID] = g
		} else {
			result[r.UID] = dto.UserGroups{
				UID:      r.UID,
				Username: r.Username,
				Groups: []dto.GroupInfo{
					{GID: r.GID, GroupName: r.GroupName, Role: r.Role},
				},
			}
		}
	}
	return result
}

func (s *UserGroupService) FormatByGID(records []models.UserGroupView) map[uint]dto.GroupUsers {
	result := make(map[uint]dto.GroupUsers)

	for _, r := range records {
		if g, exists := result[r.GID]; exists {
			g.Users = append(g.Users, dto.UserInfo{
				UID:      r.UID,
				Username: r.Username,
				Role:     r.Role,
			})
			result[r.GID] = g
		} else {
			result[r.GID] = dto.GroupUsers{
				GID:       r.GID,
				GroupName: r.GroupName,
				Users: []dto.UserInfo{
					{UID: r.UID, Username: r.Username, Role: r.Role},
				},
			}
		}
	}
	return result
}
