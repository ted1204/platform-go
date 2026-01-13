package application

import (
	"errors"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/config"
	"github.com/linskybing/platform-go/internal/domain/group"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/pkg/k8s"
	"github.com/linskybing/platform-go/pkg/utils"
)

var (
	ErrReservedUser                = errors.New("cannot modify reserved user & group 'admin & super'")
	ErrCannotRemoveAdminFromSuper  = errors.New("cannot remove admin user from " + config.ReservedGroupName + " group")
	ErrCannotDowngradeAdminInSuper = errors.New("cannot downgrade admin user role in " + config.ReservedGroupName + " group")
)

type UserGroupService struct {
	Repos *repository.Repos
}

func NewUserGroupService(repos *repository.Repos) *UserGroupService {
	return &UserGroupService{
		Repos: repos,
	}
}

func (s *UserGroupService) AllocateGroupResource(gid uint, userName string) error {
	projects, err := s.Repos.Project.ListProjectsByGroup(gid)
	if err != nil {
		return fmt.Errorf("failed to list projects for group %d: %w", gid, err)
	}

	safeUsername := k8s.ToSafeK8sName(userName)

	for _, project := range projects {
		ns := k8s.FormatNamespaceName(project.PID, safeUsername)

		log.Printf("[Allocate] Ensuring namespace %s exists for user %s", ns, safeUsername)
		if err := k8s.EnsureNamespaceExists(ns); err != nil {
			log.Printf("[Error] Failed to create namespace %s: %v", ns, err)
			continue
		}

	}

	return nil
}

func (s *UserGroupService) RemoveGroupResource(gid uint, userName string) error {
	projects, err := s.Repos.Project.ListProjectsByGroup(gid)
	if err != nil {
		return fmt.Errorf("failed to list projects for group %d: %w", gid, err)
	}

	safeUsername := k8s.ToSafeK8sName(userName)
	var lastErr error

	for _, project := range projects {
		ns := k8s.FormatNamespaceName(project.PID, safeUsername)

		log.Printf("[Cleanup] Removing resource namespace %s for user %s", ns, safeUsername)

		if err := k8s.DeleteNamespace(ns); err != nil {
			log.Printf("[Warning] Failed to delete namespace %s: %v", ns, err)
			lastErr = err
			continue
		}
	}

	return lastErr
}

func (s *UserGroupService) CreateUserGroup(c *gin.Context, userGroup *group.UserGroup) (*group.UserGroup, error) {
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

func (s *UserGroupService) UpdateUserGroup(c *gin.Context, userGroup *group.UserGroup, existing group.UserGroup) (*group.UserGroup, error) {
	// Check if trying to downgrade admin user in super group
	groupData, err := s.Repos.Group.GetGroupByID(userGroup.GID)
	if err == nil && groupData.GroupName == config.ReservedGroupName {
		username, err := s.Repos.User.GetUsernameByID(userGroup.UID)
		if err == nil && username == config.ReservedAdminUsername {
			// Check if role is being changed to something other than admin
			if userGroup.Role != "admin" && existing.Role == "admin" {
				return nil, ErrCannotDowngradeAdminInSuper
			}
		}
	}

	if err := s.Repos.UserGroup.UpdateUserGroup(userGroup); err != nil {
		return nil, err
	}

	utils.LogAuditWithConsole(c, "update", "user_group",
		fmt.Sprintf("u_id=%d,g_id=%d", userGroup.UID, userGroup.GID),
		existing, *userGroup, "", s.Repos.Audit)

	return userGroup, nil
}

func (s *UserGroupService) DeleteUserGroup(c *gin.Context, uid, gid uint) error {
	oldUserGroup, err := s.Repos.UserGroup.GetUserGroup(uid, gid)
	if err != nil {
		return err
	}

	// Check if trying to remove admin user from super group
	groupData, err := s.Repos.Group.GetGroupByID(gid)
	if err == nil && groupData.GroupName == config.ReservedGroupName {
		username, err := s.Repos.User.GetUsernameByID(uid)
		if err == nil && username == config.ReservedAdminUsername {
			return ErrCannotRemoveAdminFromSuper
		}
	}

	// Check if this is the admin user or super group (legacy check, kept for backward compatibility)
	if uid == 1 && gid == 1 {
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

	utils.LogAuditWithConsole(c, "delete", "user_group",
		fmt.Sprintf("u_id=%d,g_id=%d", uid, gid),
		oldUserGroup, nil, "", s.Repos.Audit)

	return nil
}

func (s *UserGroupService) GetUserGroup(uid, gid uint) (group.UserGroup, error) {
	return s.Repos.UserGroup.GetUserGroup(uid, gid)
}

func (s *UserGroupService) GetUserGroupsByUID(uid uint) ([]group.UserGroup, error) {
	return s.Repos.UserGroup.GetUserGroupsByUID(uid)
}

func (s *UserGroupService) GetUserGroupsByGID(gid uint) ([]group.UserGroup, error) {
	return s.Repos.UserGroup.GetUserGroupsByGID(gid)
}

func (s *UserGroupService) FormatByUID(records []group.UserGroup) map[uint]map[string]interface{} {
	result := make(map[uint]map[string]interface{})

	for _, r := range records {
		// Get group name for this group
		groupData, err := s.Repos.Group.GetGroupByID(r.GID)
		groupName := ""
		if err == nil {
			groupName = groupData.GroupName
		}

		groupInfo := map[string]interface{}{
			"GID":       r.GID,
			"GroupName": groupName,
			"Role":      r.Role,
		}

		if u, exists := result[r.UID]; exists {
			// Append to existing groups array
			groups := u["Groups"].([]map[string]interface{})
			groups = append(groups, groupInfo)
			u["Groups"] = groups
		} else {
			// Get username
			username, err := s.Repos.User.GetUsernameByID(r.UID)
			if err != nil {
				username = "" // If we can't get the username, use empty string
			}

			// Create new entry with groups array
			result[r.UID] = map[string]interface{}{
				"UID":      r.UID,
				"UserName": username,
				"Groups":   []map[string]interface{}{groupInfo},
			}
		}
	}
	return result
}

func (s *UserGroupService) FormatByGID(records []group.UserGroup) map[uint]map[string]interface{} {
	result := make(map[uint]map[string]interface{})

	for _, r := range records {
		// Get username for this user
		username, err := s.Repos.User.GetUsernameByID(r.UID)
		if err != nil {
			username = "" // If we can't get the username, use empty string
		}

		userInfo := map[string]interface{}{
			"UID":      r.UID,
			"Username": username,
			"Role":     r.Role,
		}

		if g, exists := result[r.GID]; exists {
			// Append to existing users array
			users := g["Users"].([]map[string]interface{})
			users = append(users, userInfo)
			g["Users"] = users
		} else {
			// Get group name
			groupData, err := s.Repos.Group.GetGroupByID(r.GID)
			groupName := ""
			if err == nil {
				groupName = groupData.GroupName
			}

			// Create new entry with users array
			result[r.GID] = map[string]interface{}{
				"GID":       r.GID,
				"GroupName": groupName,
				"Users":     []map[string]interface{}{userInfo},
			}
		}
	}
	return result
}
