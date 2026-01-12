package application

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/domain/view"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/pkg/k8s"
	"github.com/linskybing/platform-go/pkg/utils"
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
	p, err := s.Repos.Project.ListProjectsByUserID(id)
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
	// Validate that the group exists
	if _, err := s.Repos.Group.GetGroupByID(input.GID); err != nil {
		return nil, fmt.Errorf("group with ID %d not found", input.GID)
	}

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

	utils.LogAuditWithConsole(c, "create", "project", fmt.Sprintf("p_id=%d", p.PID), nil, p, "", s.Repos.Audit)

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
		utils.LogAuditWithConsole(c, "update", "project", fmt.Sprintf("p_id=%d", p.PID), oldProject, p, "", s.Repos.Audit)
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
		utils.LogAuditWithConsole(c, "delete", "project", fmt.Sprintf("p_id=%d", project.PID), project, nil, "", s.Repos.Audit)
	}
	return err
}

func (s *ProjectService) ListProjects() ([]project.Project, error) {
	return s.Repos.Project.ListProjects()
}

func (s *ProjectService) RemoveProjectResources(projectID uint) error {
	users, err := s.Repos.User.ListUsersByProjectID(projectID)
	if err != nil {
		return err
	}

	for _, user := range users {
		ns := k8s.FormatNamespaceName(projectID, user.Username)

		if err := k8s.DeleteNamespace(ns); err != nil {
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
	role, err := s.Repos.UserGroup.GetUserRoleInGroup(uid, gid)
	if err != nil {
		// Default to "user" for safety if no specific role is found in that group
		return "user", nil
	}

	return role, nil
}
