package application

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/config"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/domain/view"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var ErrProjectNotFound = errors.New("project not found")

type ProjectService struct {
	Repos *repository.Repos
}

func NewProjectService(repos *repository.Repos) *ProjectService {
	return &ProjectService{
		Repos: repos,
	}
}

func (s *ProjectService) GetProject(id uint) (*project.Project, error) {
	p, err := s.Repos.Project.GetProjectByID(id)
	if err != nil {
		return nil, ErrProjectNotFound
	}
	return &p, nil
}

func (s *ProjectService) GetProjectsByUser(id uint) ([]view.ProjectUserView, error) {
	p, err := s.Repos.View.ListProjectsByUserID(id)
	if err != nil {
		return nil, ErrProjectNotFound
	}
	return p, nil
}

func (s *ProjectService) GroupProjectsByGID(records []view.ProjectUserView) map[string]map[string]interface{} {
	grouped := make(map[string]map[string]interface{})

	for _, r := range records {
		key := strconv.Itoa(int(r.GID))
		if _, exists := grouped[key]; !exists {
			grouped[key] = map[string]interface{}{
				"GroupName": r.GroupName,
				"Projects":  []map[string]interface{}{},
			}
		}
		projects := grouped[key]["Projects"].([]map[string]interface{})
		projects = append(projects, map[string]interface{}{
			"PID":         r.PID,
			"ProjectName": r.ProjectName,
		})
		grouped[key]["Projects"] = projects
	}

	return grouped
}

func (s *ProjectService) GetProjectsByGroupId(id uint) ([]project.Project, error) {
	return s.Repos.Project.ListProjectsByGroup(id)
}

func (s *ProjectService) CreateProject(c *gin.Context, input project.CreateProjectDTO) (*project.Project, error) {
	p := &project.Project{
		ProjectName: input.ProjectName,
		GID:         input.GID,
	}
	if input.Description != nil {
		p.Description = *input.Description
	}
	if input.GPUQuota != nil {
		p.GPUQuota = *input.GPUQuota
	}
	if input.GPUAccess != nil {
		p.GPUAccess = *input.GPUAccess
	}
	if input.MPSMemory != nil {
		p.MPSMemory = *input.MPSMemory
	}
	err := s.Repos.Project.CreateProject(p)
	if err != nil {
		return nil, err
	}

	// Sanity check: Verify GORM properly populated the PID
	if p.PID == 0 {
		fmt.Fprintf(os.Stderr, "ERROR CreateProject: GORM did not populate p.PID after CREATE. This indicates a database or driver issue.\n")
		return nil, errors.New("failed to get project ID from database")
	}

	err = s.AllocateProjectResources(p.PID)
	if err != nil {
		// Resource allocation failed. Delete the project to maintain consistency.
		// This ensures the project doesn't exist in a partially-initialized state.
		if delErr := s.Repos.Project.DeleteProject(p.PID); delErr != nil {
			// Log but don't overwrite the original error
			fmt.Fprintf(os.Stderr, "ERROR: Failed to clean up project %d after AllocateProjectResources failed: %v\n", p.PID, delErr)
		}
		return nil, fmt.Errorf("failed to allocate project resources: %w", err)
	}

	go utils.LogAuditWithConsole(c, "create", "project", fmt.Sprintf("p_id=%d", p.PID), nil, p, "", s.Repos.Audit)

	return p, nil
}

func (s *ProjectService) UpdateProject(c *gin.Context, id uint, input project.UpdateProjectDTO) (*project.Project, error) {
	p, err := s.Repos.Project.GetProjectByID(id)
	if err != nil {
		return nil, ErrProjectNotFound
	}

	oldProject := p

	if input.ProjectName != nil {
		p.ProjectName = *input.ProjectName
	}
	if input.Description != nil {
		p.Description = *input.Description
	}
	if input.GID != nil {
		p.GID = *input.GID
	}
	if input.GPUQuota != nil {
		p.GPUQuota = *input.GPUQuota
	}
	if input.GPUAccess != nil {
		p.GPUAccess = *input.GPUAccess
	}
	if input.MPSMemory != nil {
		p.MPSMemory = *input.MPSMemory
	}

	err = s.Repos.Project.UpdateProject(&p)
	if err == nil {
		go utils.LogAuditWithConsole(c, "update", "project", fmt.Sprintf("p_id=%d", p.PID), oldProject, p, "", s.Repos.Audit)
	}

	return &p, err
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

func (s *ProjectService) ListProjects() ([]project.Project, error) {
	return s.Repos.Project.ListProjects()
}

func (s *ProjectService) AllocateProjectResources(projectID uint) error {
	// Ensure project-level shared storage hub (namespace + NFS gateway) exists
	project, err := s.Repos.Project.GetProjectByID(projectID)
	if err != nil {
		return err
	}

	projectNamespace := utils.GenerateSafeResourceName("project", project.ProjectName, project.PID)
	pvcName := fmt.Sprintf("project-%d-disk", projectID)

	if err := utils.CreateNamespace(projectNamespace); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "already exist") {
			return fmt.Errorf("failed to create project namespace: %w", err)
		}
	}

	if err := utils.CreateHubPVC(projectNamespace, pvcName, config.DefaultStorageClassName, config.UserPVSize); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create project hub pvc: %w", err)
		}
	}

	if err := utils.CreateNFSDeployment(projectNamespace, pvcName); err != nil {
		return fmt.Errorf("failed to create project nfs deployment: %w", err)
	}

	if err := utils.CreateNFSServiceWithName(projectNamespace, config.ProjectNfsServiceName); err != nil {
		return fmt.Errorf("failed to create project nfs service: %w", err)
	}

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

func (s *ProjectService) CreateProjectPVC(projectID uint, input project.CreateProjectPVCDTO) error {
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
