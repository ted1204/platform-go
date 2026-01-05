package application

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/config"
	"github.com/linskybing/platform-go/internal/domain/group"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/pkg/utils"
)

var ErrReservedGroupName = errors.New("cannot use reserved group name '" + config.ReservedGroupName + "'")

type GroupService struct {
	Repos *repository.Repos
}

func NewGroupService(repos *repository.Repos) *GroupService {
	return &GroupService{
		Repos: repos,
	}
}

func (s *GroupService) ListGroups() ([]group.Group, error) {
	return s.Repos.Group.GetAllGroups()
}

func (s *GroupService) GetGroup(id uint) (group.Group, error) {
	return s.Repos.Group.GetGroupByID(id)
}

func (s *GroupService) CreateGroup(c *gin.Context, input group.GroupCreateDTO) (group.Group, error) {
	if input.GroupName == config.ReservedGroupName {
		return group.Group{}, ErrReservedGroupName
	}

	grp := group.Group{
		GroupName: input.GroupName,
	}
	if input.Description != nil {
		grp.Description = *input.Description
	}

	err := s.Repos.Group.CreateGroup(&grp)
	if err != nil {
		return group.Group{}, err
	}
	utils.LogAuditWithConsole(c, "create", "group", fmt.Sprintf("g_id=%d", grp.GID), nil, grp, "", s.Repos.Audit)

	return grp, nil
}

func (s *GroupService) UpdateGroup(c *gin.Context, id uint, input group.GroupUpdateDTO) (group.Group, error) {
	grp, err := s.Repos.Group.GetGroupByID(id)
	if err != nil {
		return group.Group{}, err
	}

	// Cannot modify the reserved super group's name
	if grp.GroupName == config.ReservedGroupName && input.GroupName != nil {
		return group.Group{}, ErrReservedGroupName
	}

	oldGroup := grp

	if input.GroupName != nil {
		// Prevent updating reserved group name (both TO and FROM)
		if grp.GroupName == config.ReservedGroupName {
			return group.Group{}, ErrReservedGroupName
		}
		if *input.GroupName == config.ReservedGroupName {
			return group.Group{}, ErrReservedGroupName
		}
		grp.GroupName = *input.GroupName
	}
	if input.Description != nil {
		grp.Description = *input.Description
	}

	err = s.Repos.Group.UpdateGroup(&grp)
	if err != nil {
		return group.Group{}, err
	}

	utils.LogAuditWithConsole(c, "update", "group", fmt.Sprintf("g_id=%d", grp.GID), oldGroup, grp, "", s.Repos.Audit)

	return grp, nil
}

func (s *GroupService) DeleteGroup(c *gin.Context, id uint) error {
	group, err := s.Repos.Group.GetGroupByID(id)
	if err != nil {
		return err
	}

	if group.GroupName == config.ReservedGroupName {
		return ErrReservedGroupName
	}

	err = s.Repos.Group.DeleteGroup(id)
	if err != nil {
		return err
	}

	utils.LogAuditWithConsole(c, "delete", "group", fmt.Sprintf("g_id=%d", group.GID), group, nil, "", s.Repos.Audit)

	return nil
}
