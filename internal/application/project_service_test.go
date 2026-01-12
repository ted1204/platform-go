package application_test

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/linskybing/platform-go/internal/application"
	"github.com/linskybing/platform-go/internal/domain/group"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/domain/view"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/internal/repository/mock"
	"github.com/linskybing/platform-go/pkg/utils"
)

func setupProjectMocks(t *testing.T) (*application.ProjectService,
	*mock.MockProjectRepo,
	*mock.MockAuditRepo,
	*gin.Context) {

	ctrl := gomock.NewController(t)
	t.Cleanup(func() { ctrl.Finish() })

	mockProject := mock.NewMockProjectRepo(ctrl)
	mockAudit := mock.NewMockAuditRepo(ctrl)
	mockGroup := mock.NewMockGroupRepo(ctrl)
	mockUser := mock.NewMockUserRepo(ctrl)

	repos := &repository.Repos{
		Project: mockProject,
		Audit:   mockAudit,
		Group:   mockGroup,
		User:    mockUser,
	}

	svc := application.NewProjectService(repos)
	c, _ := gin.CreateTestContext(nil)

	// mock utils globally
	utils.LogAuditWithConsole = func(c *gin.Context, action, resourceType, resourceID string, oldData, newData interface{}, msg string, repos repository.AuditRepo) {
	}

	// default user repo behavior to avoid nil deref in AllocateProjectResources
	mockUser.EXPECT().ListUsersByProjectID(gomock.Any()).Return([]view.ProjectUserView{}, nil).AnyTimes()

	return svc, mockProject, mockAudit, c
}

func TestProjectServiceCRUD(t *testing.T) {
	svc, mockProject, _, c := setupProjectMocks(t)

	// Get mockGroup from the service's repositories
	mockGroup := svc.Repos.Group.(*mock.MockGroupRepo)

	t.Run("CreateProject success", func(t *testing.T) {
		input := project.CreateProjectDTO{ProjectName: "proj1", GID: 1}

		// First, mock GetGroupByID (called for validation)
		mockGroup.EXPECT().GetGroupByID(uint(1)).Return(group.Group{GID: 1, GroupName: "test-group"}, nil)

		mockProject.EXPECT().CreateProject(gomock.Any()).Do(func(p *project.Project) {
			// Simulate GORM's behavior of setting the PID after successful CREATE
			p.PID = 1
		}).Return(nil)

		// CreateProject calls AllocateProjectResources which may call GetProjectByID; make optional
		mockProject.EXPECT().GetProjectByID(uint(1)).Return(project.Project{PID: 1, ProjectName: "proj1", GID: 1}, nil).AnyTimes()

		proj, err := svc.CreateProject(c, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if proj.ProjectName != "proj1" {
			t.Fatalf("expected proj1, got %s", proj.ProjectName)
		}
		if proj.PID != 1 {
			t.Fatalf("expected PID 1, got %d", proj.PID)
		}
	})

	t.Run("CreateProject error handling", func(t *testing.T) {
		input := project.CreateProjectDTO{ProjectName: "proj2", GID: 1}
		expectedErr := errors.New("database error")

		// Mock GetGroupByID for validation
		mockGroup.EXPECT().GetGroupByID(uint(1)).Return(group.Group{GID: 1, GroupName: "test-group"}, nil)
		mockProject.EXPECT().CreateProject(gomock.Any()).Return(expectedErr)

		project, err := svc.CreateProject(c, input)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if err.Error() != "database error" {
			t.Fatalf("expected 'database error', got %v", err)
		}
		if project != nil {
			t.Fatalf("expected nil project on error, got %v", project)
		}
	})

	// Resource allocation now happens when a user joins a project (via UserGroupService).
	// The CreateProject flow does not attempt to allocate namespaces/PVCs anymore.

	t.Run("UpdateProject success", func(t *testing.T) {
		oldProject := project.Project{PID: 1, ProjectName: "old", GID: 1}
		mockProject.EXPECT().GetProjectByID(uint(1)).Return(oldProject, nil).AnyTimes()
		mockProject.EXPECT().UpdateProject(gomock.Any()).Return(nil)

		newName := "new"
		input := project.UpdateProjectDTO{ProjectName: &newName}
		project, err := svc.UpdateProject(c, 1, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if project.ProjectName != "new" {
			t.Fatalf("expected new, got %s", project.ProjectName)
		}
	})

	t.Run("UpdateProject not found", func(t *testing.T) {
		mockProject.EXPECT().GetProjectByID(uint(99)).Return(project.Project{}, errors.New("not found")).AnyTimes()
		newName := "test"
		input := project.UpdateProjectDTO{ProjectName: &newName}
		_, err := svc.UpdateProject(c, 99, input)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("DeleteProject success", func(t *testing.T) {
		proj := project.Project{PID: 1, ProjectName: "proj1", GID: 1}
		mockProject.EXPECT().GetProjectByID(uint(1)).Return(proj, nil).AnyTimes()
		mockProject.EXPECT().DeleteProject(uint(1)).Return(nil)

		err := svc.DeleteProject(c, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("DeleteProject fails if project not found", func(t *testing.T) {
		mockProject.EXPECT().GetProjectByID(uint(99)).Return(project.Project{}, errors.New("not found")).AnyTimes()
		err := svc.DeleteProject(c, 99)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestProjectServiceRead(t *testing.T) {
	svc, mockProject, _, _ := setupProjectMocks(t)

	t.Run("GetProjects success", func(t *testing.T) {
		projects := []project.Project{{PID: 1, ProjectName: "p1"}}
		mockProject.EXPECT().ListProjects().Return(projects, nil)

		res, err := svc.ListProjects()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 project, got %d", len(res))
		}
	})

	t.Run("GetProjectsByUser success", func(t *testing.T) {
		projects := []view.ProjectUserView{{PID: 1, ProjectName: "p1"}}
		mockProject.EXPECT().ListProjectsByUserID(uint(1)).Return(projects, nil)

		res, err := svc.GetProjectsByUser(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 project, got %d", len(res))
		}
	})

	t.Run("GetProjectByID success", func(t *testing.T) {
		proj := project.Project{PID: 1, ProjectName: "p1"}
		mockProject.EXPECT().GetProjectByID(uint(1)).Return(proj, nil).AnyTimes()

		res, err := svc.GetProject(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.ProjectName != "p1" {
			t.Fatalf("expected p1, got %s", res.ProjectName)
		}
	})

	t.Run("GetProjectByID not found", func(t *testing.T) {
		mockProject.EXPECT().GetProjectByID(uint(99)).Return(project.Project{}, errors.New("not found")).AnyTimes()

		_, err := svc.GetProject(99)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
