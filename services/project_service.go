package services

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/config"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/models"
	"github.com/linskybing/platform-go/repositories"
	"github.com/linskybing/platform-go/utils"
)

var ErrProjectNotFound = errors.New("project not found")

func GetProject(id uint) (models.Project, error) {
	project, err := repositories.GetProjectByID(id)
	if err != nil {
		return models.Project{}, ErrProjectNotFound
	}
	return project, nil
}

func GetProjectsByGroupId(id uint) ([]models.Project, error) {
	return repositories.ListProjectsByGroup(id)
}

func CreateProject(c *gin.Context, input dto.CreateProjectDTO) (models.Project, error) {
	project := models.Project{
		ProjectName: input.ProjectName,
		GID:         input.GID,
	}
	if input.Description != nil {
		project.Description = *input.Description
	}
	err := repositories.CreateProject(&project)
	if err == nil {
		utils.LogAuditWithConsole(c, "create", "project", fmt.Sprintf("p_id=%d", project.PID), nil, project, "")
	}

	if err := AllocateProjectResources(project.PID); err != nil {
		return project, err
	}

	return project, err
}

func UpdateProject(c *gin.Context, id uint, input dto.UpdateProjectDTO) (models.Project, error) {
	project, err := repositories.GetProjectByID(id)
	if err != nil {
		return models.Project{}, ErrProjectNotFound
	}

	oldProject := project

	if input.ProjectName != nil {
		project.ProjectName = *input.ProjectName
	}
	if input.Description != nil {
		project.Description = *input.Description
	}
	if input.GID != nil {
		project.GID = *input.GID
	}

	err = repositories.UpdateProject(&project)
	if err == nil {
		utils.LogAuditWithConsole(c, "update", "project", fmt.Sprintf("p_id=%d", project.PID), oldProject, project, "")
	}

	return project, err
}

func DeleteProject(c *gin.Context, id uint) error {
	project, err := repositories.GetProjectByID(id)
	if err != nil {
		return ErrProjectNotFound
	}

	if err := RemoveProjectResources(id); err != nil {
		return err
	}

	err = repositories.DeleteProject(id)
	if err == nil {
		utils.LogAuditWithConsole(c, "delete", "project", fmt.Sprintf("p_id=%d", project.PID), project, nil, "")
	}
	return err
}

func ListProjects() ([]models.Project, error) {
	return repositories.ListProjects()
}

func AllocateProjectResources(projectID uint) error {
	users, err := repositories.ListUsersByProjectID(projectID)
	if err != nil {
		return err
	}

	for _, user := range users {
		ns := utils.FormatNamespaceName(projectID, user.Username)

		if err := utils.CreateNamespace(ns); err != nil {
			return fmt.Errorf("failed to create namespace for %s: %w", user.Username, err)
		}

		if err := utils.CreatePVC(ns, config.DefaultStorageName, config.DefaultStorageClassName, config.DefaultStorageSize); err != nil {
			return fmt.Errorf("failed to create PVC for %s: %w", user.Username, err)
		}
	}

	return nil
}

func RemoveProjectResources(projectID uint) error {
	users, err := repositories.ListUsersByProjectID(projectID)
	if err != nil {
		return err
	}

	for _, user := range users {
		ns := utils.FormatNamespaceName(projectID, user.Username)

		if err := utils.DeleteNamespace(ns); err != nil {
			return fmt.Errorf("failed to delete namespace %s: %w", ns, err)
		}
	}

	return nil
}
