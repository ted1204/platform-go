package services

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/models"
	"github.com/linskybing/platform-go/src/repositories"
	"github.com/linskybing/platform-go/src/utils"
)

var ErrReservedGroupName = errors.New("cannot use reserved group name 'super'")

type GroupService struct {
	Repos *repositories.Repos
}

func NewGroupService(repos *repositories.Repos) *GroupService {
	return &GroupService{
		Repos: repos,
	}
}

func (s *GroupService) ListGroups() ([]models.Group, error) {
	return s.Repos.Group.GetAllGroups()
}

func (s *GroupService) GetGroup(id uint) (models.Group, error) {
	return s.Repos.Group.GetGroupByID(id)
}

func (s *GroupService) CreateGroup(c *gin.Context, input dto.GroupCreateDTO) (models.Group, error) {
	if input.GroupName == "super" {
		return models.Group{}, ErrReservedGroupName
	}

	group := models.Group{
		GroupName: input.GroupName,
	}
	if input.Description != nil {
		group.Description = *input.Description
	}

	err := s.Repos.Group.CreateGroup(&group)
	if err != nil {
		return models.Group{}, err
	}
	go utils.LogAuditWithConsole(c, "create", "group", fmt.Sprintf("g_id=%d", group.GID), nil, group, "", s.Repos.Audit)

	return group, nil
}

func (s *GroupService) UpdateGroup(c *gin.Context, id uint, input dto.GroupUpdateDTO) (models.Group, error) {
	group, err := s.Repos.Group.GetGroupByID(id)
	if err != nil {
		return models.Group{}, err
	}

	oldGroup := group

	if input.GroupName != nil {
		if *input.GroupName == "super" {
			return models.Group{}, ErrReservedGroupName
		}
		group.GroupName = *input.GroupName
	}
	if input.Description != nil {
		group.Description = *input.Description
	}

	err = s.Repos.Group.UpdateGroup(&group)
	if err != nil {
		return models.Group{}, err
	}

	go utils.LogAuditWithConsole(c, "update", "group", fmt.Sprintf("g_id=%d", group.GID), oldGroup, group, "", s.Repos.Audit)

	return group, nil
}

func (s *GroupService) DeleteGroup(c *gin.Context, id uint) error {
	group, err := s.Repos.Group.GetGroupByID(id)
	if err != nil {
		return err
	}

	if group.GroupName == "super" {
		return ErrReservedGroupName
	}

	err = s.Repos.Group.DeleteGroup(id)
	if err != nil {
		return err
	}

	go utils.LogAuditWithConsole(c, "delete", "group", fmt.Sprintf("g_id=%d", group.GID), group, nil, "", s.Repos.Audit)

	return nil
}
