package services

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/src/config"
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/models"
	"github.com/linskybing/platform-go/src/repositories"
	"github.com/linskybing/platform-go/src/utils"
)

var ErrProjectNotFound = errors.New("project not found")

type ProjectService struct {
	Repos *repositories.Repos
}

func NewProjectService(repos *repositories.Repos) *ProjectService {
	return &ProjectService{
		Repos: repos,
	}
}

func (s *ProjectService) GetProject(id uint) (models.Project, error) {
	project, err := s.Repos.Project.GetProjectByID(id)
	if err != nil {
		return models.Project{}, ErrProjectNotFound
	}
	return project, nil
}

func (s *ProjectService) GetProjectsByUser(id uint) ([]models.ProjectUserView, error) {
	project, err := s.Repos.View.ListProjectsByUserID(id)
	if err != nil {
		return nil, ErrProjectNotFound
	}
	return project, nil
}

func (s *ProjectService) GroupProjectsByGID(records []models.ProjectUserView) map[string]dto.GroupProjects {
	grouped := make(map[string]dto.GroupProjects)

	for _, r := range records {
		key := strconv.Itoa(int(r.GID))
		gp, exists := grouped[key]
		if !exists {
			gp = dto.GroupProjects{
				GroupName: r.GroupName,
				Projects:  []dto.SimpleProjectInfo{},
			}
		}
		gp.Projects = append(gp.Projects, dto.SimpleProjectInfo{
			PID:         r.PID,
			ProjectName: r.ProjectName,
		})
		grouped[key] = gp
	}

	return grouped
}

func (s *ProjectService) GetProjectsByGroupId(id uint) ([]models.Project, error) {
	return s.Repos.Project.ListProjectsByGroup(id)
}

func (s *ProjectService) CreateProject(c *gin.Context, input dto.CreateProjectDTO) (models.Project, error) {
	project := models.Project{
		ProjectName: input.ProjectName,
		GID:         input.GID,
	}
	if input.Description != nil {
		project.Description = *input.Description
	}
	if input.GPUQuota != nil {
		project.GPUQuota = *input.GPUQuota
	}
	if input.GPUAccess != nil {
		project.GPUAccess = *input.GPUAccess
	}
	if input.MPSLimit != nil {
		project.MPSLimit = *input.MPSLimit
	}
	if input.MPSMemory != nil {
		project.MPSMemory = *input.MPSMemory
	}
	err := s.Repos.Project.CreateProject(&project)
	err = s.AllocateProjectResources(project.PID)
	if err == nil {
		go utils.LogAuditWithConsole(c, "create", "project", fmt.Sprintf("p_id=%d", project.PID), nil, project, "", s.Repos.Audit)
	}

	return project, err
}

func (s *ProjectService) UpdateProject(c *gin.Context, id uint, input dto.UpdateProjectDTO) (models.Project, error) {
	project, err := s.Repos.Project.GetProjectByID(id)
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
	if input.GPUQuota != nil {
		project.GPUQuota = *input.GPUQuota
	}
	if input.GPUAccess != nil {
		project.GPUAccess = *input.GPUAccess
	}
	if input.MPSLimit != nil {
		project.MPSLimit = *input.MPSLimit
	}
	if input.MPSMemory != nil {
		project.MPSMemory = *input.MPSMemory
	}

	err = s.Repos.Project.UpdateProject(&project)
	if err == nil {
		go utils.LogAuditWithConsole(c, "update", "project", fmt.Sprintf("p_id=%d", project.PID), oldProject, project, "", s.Repos.Audit)
	}

	return project, err
}

func (s *ProjectService) DeleteProject(c *gin.Context, id uint) error {
	project, err := s.Repos.Project.GetProjectByID(id)
	if err != nil {
		return ErrProjectNotFound
	}

	_ = s.RemoveProjectResources(id)

	err = s.Repos.Project.DeleteProject(id)
	if err == nil {
		go utils.LogAuditWithConsole(c, "delete", "project", fmt.Sprintf("p_id=%d", project.PID), project, nil, "", s.Repos.Audit)
	}
	return err
}

func (s *ProjectService) ListProjects() ([]models.Project, error) {
	return s.Repos.Project.ListProjects()
}

func (s *ProjectService) AllocateProjectResources(projectID uint) error {
	users, err := s.Repos.View.ListUsersByProjectID(projectID)
	if err != nil {
		return err
	}

	for _, user := range users {
		ns := utils.FormatNamespaceName(projectID, user.Username)

		if err := utils.CreateNamespace(ns); err != nil {
			return fmt.Errorf("failed to create namespace for %s: %w", user.Username, err)
		}

		// // Create User PV (Static Provisioning pointing to shared volume)
		// // We use a consistent volume handle name based on username to ensure all projects share the same storage
		// pvName := fmt.Sprintf("pv-user-%s-proj-%d", user.Username, projectID)
		// volumeHandle := fmt.Sprintf("vol-user-%s", user.Username)

		// // If using HostPath (not longhorn), we need a path.
		// // If using Longhorn, we use volumeHandle.
		// // Since config.UserPVPath was removed, we assume Longhorn or use a default path for HostPath fallback.
		// path := volumeHandle
		// if config.DefaultStorageClassName != "longhorn" {
		// 	path = "/mnt/data/users/" + user.Username
		// }

		// if err := utils.CreatePV(pvName, config.DefaultStorageClassName, config.UserPVSize, path); err != nil {
		// 	return fmt.Errorf("failed to create PV for %s: %w", user.Username, err)
		// }

		// // Create PVC bound to the specific PV
		// if err := utils.CreateBoundPVC(ns, config.DefaultStorageName, config.DefaultStorageClassName, config.UserPVSize, pvName); err != nil {
		// 	return fmt.Errorf("failed to create PVC for %s: %w", user.Username, err)
		// }
	}

	return nil
}

func (s *ProjectService) CreateProjectPVC(projectID uint, input dto.CreateProjectPVCDTO) error {
	// 3. List all users in the project
	users, err := s.Repos.View.ListUsersByProjectID(projectID)
	if err != nil {
		return err
	}

	// 4. Create PVC in each user's namespace (Dynamic Provisioning)
	// Note: Without a shared underlying volume (like NFS or pre-provisioned RWX volume),
	// these PVCs will be independent volumes.
	for _, user := range users {
		ns := utils.FormatNamespaceName(projectID, user.Username)
		if err := utils.CreatePVC(ns, input.Name, config.DefaultStorageClassName, input.Size); err != nil {
			return fmt.Errorf("failed to create shared PVC for user %s: %w", user.Username, err)
		}
	}

	return nil
}

func (s *ProjectService) RemoveProjectResources(projectID uint) error {
	users, err := s.Repos.View.ListUsersByProjectID(projectID)
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

// GetUserRoleInProjectGroup determines the user's role by looking up the group associated with the project.
func (s *ProjectService) GetUserRoleInProjectGroup(uid uint, pid uint) (string, error) {
	// 1. Get GID from project ID
	gid, err := s.Repos.Project.GetGroupIDByProjectID(pid)
	if err != nil {
		return "", err
	}

	// 2. Get role from UserGroupView via ViewRepo
	role, err := s.Repos.View.GetUserRoleInGroup(uid, gid)
	if err != nil {
		// Default to "user" for safety if no specific role is found in that group
		return "user", nil
	}

	return role, nil
}
