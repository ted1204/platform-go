package services_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/models"
	"github.com/linskybing/platform-go/src/repositories"
	"github.com/linskybing/platform-go/src/repositories/mock_repositories"
	"github.com/linskybing/platform-go/src/services"
	"github.com/linskybing/platform-go/src/utils"
)

func setupProjectMocks(t *testing.T) (*services.ProjectService,
	*mock_repositories.MockProjectRepo,
	*mock_repositories.MockViewRepo,
	*mock_repositories.MockAuditRepo,
	*gin.Context) {

	ctrl := gomock.NewController(t)
	t.Cleanup(func() { ctrl.Finish() })

	mockProject := mock_repositories.NewMockProjectRepo(ctrl)
	mockView := mock_repositories.NewMockViewRepo(ctrl)
	mockAudit := mock_repositories.NewMockAuditRepo(ctrl)

	repos := &repositories.Repos{
		Project: mockProject,
		View:    mockView,
		Audit:   mockAudit,
	}

	svc := services.NewProjectService(repos)
	c, _ := gin.CreateTestContext(nil)

	// mock utils globally
	utils.LogAuditWithConsole = func(c *gin.Context, action, resourceType, resourceID string, oldData, newData interface{}, msg string, repos repositories.AuditRepo) {
	}
	utils.FormatNamespaceName = func(pid uint, username string) string { return fmt.Sprintf("ns-%d-%s", pid, username) }
	utils.CreateNamespace = func(ns string) error { return nil }
	utils.CreatePVC = func(ns, name, class, size string) error { return nil }
	utils.DeleteNamespace = func(ns string) error { return nil }

	return svc, mockProject, mockView, mockAudit, c
}

func TestProjectServiceCRUD(t *testing.T) {
	svc, mockProject, mockView, _, c := setupProjectMocks(t)

	t.Run("CreateProject success", func(t *testing.T) {
		input := dto.CreateProjectDTO{ProjectName: "proj1", GID: 1}

		mockProject.EXPECT().CreateProject(gomock.Any()).Return(nil)
		mockView.EXPECT().ListUsersByProjectID(gomock.Any()).Return([]models.ProjectUserView{
			{Username: "user1"},
			{Username: "user2"},
		}, nil)

		project, err := svc.CreateProject(c, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if project.ProjectName != "proj1" {
			t.Fatalf("expected proj1, got %s", project.ProjectName)
		}
	})

	t.Run("CreateProject fails on resource allocation", func(t *testing.T) {
		input := dto.CreateProjectDTO{ProjectName: "proj2", GID: 1}
		mockProject.EXPECT().CreateProject(gomock.Any()).Return(nil)
		mockView.EXPECT().ListUsersByProjectID(gomock.Any()).Return(nil, errors.New("list users failed"))

		_, err := svc.CreateProject(c, input)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("UpdateProject success", func(t *testing.T) {
		oldProject := models.Project{PID: 1, ProjectName: "old", GID: 1}
		mockProject.EXPECT().GetProjectByID(uint(1)).Return(oldProject, nil)
		mockProject.EXPECT().UpdateProject(gomock.Any()).Return(nil)

		newName := "new"
		input := dto.UpdateProjectDTO{ProjectName: &newName}
		project, err := svc.UpdateProject(c, 1, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if project.ProjectName != "new" {
			t.Fatalf("expected new, got %s", project.ProjectName)
		}
	})

	t.Run("UpdateProject not found", func(t *testing.T) {
		mockProject.EXPECT().GetProjectByID(uint(99)).Return(models.Project{}, errors.New("not found"))
		newName := "test"
		input := dto.UpdateProjectDTO{ProjectName: &newName}
		_, err := svc.UpdateProject(c, 99, input)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("DeleteProject success", func(t *testing.T) {
		project := models.Project{PID: 1, ProjectName: "proj1", GID: 1}
		mockProject.EXPECT().GetProjectByID(uint(1)).Return(project, nil)
		mockProject.EXPECT().DeleteProject(uint(1)).Return(nil)
		mockView.EXPECT().ListUsersByProjectID(uint(1)).Return([]models.ProjectUserView{{Username: "user1"}}, nil)

		err := svc.DeleteProject(c, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("DeleteProject fails if project not found", func(t *testing.T) {
		mockProject.EXPECT().GetProjectByID(uint(99)).Return(models.Project{}, errors.New("not found"))
		err := svc.DeleteProject(c, 99)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("AllocateProjectResources creates namespace & pvc", func(t *testing.T) {
		mockView.EXPECT().ListUsersByProjectID(uint(1)).Return([]models.ProjectUserView{{Username: "user1"}}, nil)
		err := svc.AllocateProjectResources(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("RemoveProjectResources deletes namespace", func(t *testing.T) {
		mockView.EXPECT().ListUsersByProjectID(uint(1)).Return([]models.ProjectUserView{{Username: "user1"}}, nil)
		err := svc.RemoveProjectResources(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestProjectServiceRead(t *testing.T) {
	svc, mockProject, mockView, _, _ := setupProjectMocks(t)

	t.Run("GetProjects success", func(t *testing.T) {
		projects := []models.Project{{PID: 1, ProjectName: "p1"}}
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
		projects := []models.ProjectUserView{{PID: 1, ProjectName: "p1"}}
		mockView.EXPECT().ListProjectsByUserID(uint(1)).Return(projects, nil)

		res, err := svc.GetProjectsByUser(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 project, got %d", len(res))
		}
	})

	t.Run("GetProjectByID success", func(t *testing.T) {
		project := models.Project{PID: 1, ProjectName: "p1"}
		mockProject.EXPECT().GetProjectByID(uint(1)).Return(project, nil)

		res, err := svc.GetProject(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.ProjectName != "p1" {
			t.Fatalf("expected p1, got %s", res.ProjectName)
		}
	})

	t.Run("GetProjectByID not found", func(t *testing.T) {
		mockProject.EXPECT().GetProjectByID(uint(99)).Return(models.Project{}, errors.New("not found"))

		_, err := svc.GetProject(99)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
