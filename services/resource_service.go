package services

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/models"
	"github.com/linskybing/platform-go/repositories"
	"github.com/linskybing/platform-go/utils"
)

var ErrResourceNotFound = errors.New("resource not found")

func ListResourcesByProjectID(projectID uint) ([]models.Resource, error) {
	return repositories.ListResourcesByProjectID(projectID)
}

func ListResourcesByConfigFileID(cfID uint) ([]models.Resource, error) {
	return repositories.ListResourcesByConfigFileID(cfID)
}

func GetResource(rid uint) (*models.Resource, error) {
	return repositories.GetResourceByID(rid)
}

func CreateResource(c *gin.Context, resource *models.Resource) (*models.Resource, error) {
	err := repositories.CreateResource(resource)
	if err != nil {
		return nil, err
	}

	userID, _ := utils.GetUserIDFromContext(c)
	_ = utils.LogAudit(c, userID, "create", "resource", resource.RID, nil, *resource, "")

	return resource, nil
}

func UpdateResource(c *gin.Context, rid uint, input dto.ResourceUpdateDTO) (*models.Resource, error) {
	existing, err := repositories.GetResourceByID(rid)
	if err != nil || existing == nil {
		return nil, ErrResourceNotFound
	}

	oldResource := *existing

	if input.Type != nil {
		existing.Type = *input.Type
	}
	if input.Name != nil {
		existing.Name = *input.Name
	}
	if input.ParsedYAML != nil {
		existing.ParsedYAML = *input.ParsedYAML
	}
	if input.Description != nil {
		existing.Description = input.Description
	}

	err = repositories.UpdateResource(existing)
	if err != nil {
		return nil, err
	}

	userID, _ := utils.GetUserIDFromContext(c)
	_ = utils.LogAudit(c, userID, "update", "resource", existing.RID, oldResource, *existing, "")

	return existing, nil
}

func DeleteResource(c *gin.Context, rid uint) error {
	resource, err := repositories.GetResourceByID(rid)
	if err != nil || resource == nil {
		return ErrResourceNotFound
	}

	err = repositories.DeleteResource(rid)
	if err != nil {
		return err
	}

	userID, _ := utils.GetUserIDFromContext(c)
	_ = utils.LogAudit(c, userID, "delete", "resource", resource.RID, *resource, nil, "")

	return nil
}
