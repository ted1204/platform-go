package services

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/linskybing/platform-go/src/models"
	"github.com/linskybing/platform-go/src/repositories"
	"github.com/linskybing/platform-go/src/repositories/mock_repositories"
	"github.com/linskybing/platform-go/src/utils"
	"github.com/stretchr/testify/assert"
)

func setupUserGroupMocks(t *testing.T) (*UserGroupService,
	*mock_repositories.MockUserGroupRepo,
	*mock_repositories.MockUserRepo,
	*mock_repositories.MockProjectRepo,
	*mock_repositories.MockAuditRepo,
	*gin.Context) {

	ctrl := gomock.NewController(t)
	t.Cleanup(func() { ctrl.Finish() })

	mockUG := mock_repositories.NewMockUserGroupRepo(ctrl)
	mockUser := mock_repositories.NewMockUserRepo(ctrl)
	mockProject := mock_repositories.NewMockProjectRepo(ctrl)
	mockAudit := mock_repositories.NewMockAuditRepo(ctrl)

	repos := &repositories.Repos{
		UserGroup: mockUG,
		User:      mockUser,
		Project:   mockProject,
		Audit:     mockAudit,
	}

	service := NewUserGroupService(repos)
	ctx, _ := gin.CreateTestContext(nil)

	// override utils
	utils.LogAuditWithConsole = func(ctx *gin.Context, action, resourceType, resourceID string,
		oldData, newData interface{}, msg string, repos repositories.AuditRepo) {
	}
	utils.FormatNamespaceName = func(pid uint, username string) string {
		return fmt.Sprintf("ns-%d-%s", pid, username)
	}
	utils.CreateNamespace = func(ns string) error { return nil }
	utils.CreatePVC = func(ns, name, class, size string) error { return nil }
	utils.DeleteNamespace = func(ns string) error { return nil }

	return service, mockUG, mockUser, mockProject, mockAudit, ctx
}

//
// --- TESTS ---
//

// ---------- CreateUserGroup ----------
func TestCreateUserGroup_Success(t *testing.T) {
	svc, ugRepo, userRepo, projectRepo, _, ctx := setupUserGroupMocks(t)

	ug := &models.UserGroup{UID: 1, GID: 2}
	projects := []models.Project{{PID: 100, ProjectName: "p1"}}

	ugRepo.EXPECT().CreateUserGroup(ug).Return(nil)
	userRepo.EXPECT().GetUsernameByID(uint(1)).Return("admin", nil)
	projectRepo.EXPECT().ListProjectsByGroup(uint(2)).Return(projects, nil)

	res, err := svc.CreateUserGroup(ctx, ug)

	assert.NoError(t, err)
	assert.Equal(t, ug, res)
}

func TestCreateUserGroup_Fail_CreateRepo(t *testing.T) {
	svc, ugRepo, _, _, _, ctx := setupUserGroupMocks(t)

	ug := &models.UserGroup{UID: 1, GID: 2}
	ugRepo.EXPECT().CreateUserGroup(ug).Return(errors.New("db error"))

	res, err := svc.CreateUserGroup(ctx, ug)

	assert.Error(t, err)
	assert.Nil(t, res)
}

func TestCreateUserGroup_Fail_GetUser(t *testing.T) {
	svc, ugRepo, userRepo, _, _, ctx := setupUserGroupMocks(t)

	ug := &models.UserGroup{UID: 1, GID: 2}
	ugRepo.EXPECT().CreateUserGroup(ug).Return(nil)
	userRepo.EXPECT().GetUsernameByID(uint(1)).Return("", errors.New("user not found"))

	res, err := svc.CreateUserGroup(ctx, ug)

	assert.Error(t, err)
	assert.Nil(t, res)
}

func TestUpdateUserGroup_Success(t *testing.T) {
	svc, ugRepo, _, _, _, ctx := setupUserGroupMocks(t)

	oldUG := models.UserGroupView{UID: 1, GID: 1, Role: "user"}
	newUG := &models.UserGroup{UID: 1, GID: 1, Role: "admin"}

	ugRepo.EXPECT().UpdateUserGroup(newUG).Return(nil)

	res, err := svc.UpdateUserGroup(ctx, newUG, oldUG)
	assert.NoError(t, err)
	assert.Equal(t, newUG, res)
}

func TestUpdateUserGroup_Fail_UpdateRepo(t *testing.T) {
	svc, ugRepo, _, _, _, ctx := setupUserGroupMocks(t)

	oldUG := models.UserGroupView{UID: 1, Username: "1234", GID: 1, GroupName: "test"}
	newUG := &models.UserGroup{UID: 1, GID: 1}

	ugRepo.EXPECT().UpdateUserGroup(newUG).Return(errors.New("update fail"))

	res, err := svc.UpdateUserGroup(ctx, newUG, oldUG)

	assert.Nil(t, res)
	assert.EqualError(t, err, "update fail")
}

// ---------- DeleteUserGroup ----------
func TestDeleteUserGroup_Success(t *testing.T) {
	svc, ugRepo, userRepo, projectRepo, _, ctx := setupUserGroupMocks(t)

	oldUG := models.UserGroupView{UID: 1, GID: 2}
	ugRepo.EXPECT().GetUserGroup(uint(1), uint(2)).Return(oldUG, nil)
	ugRepo.EXPECT().DeleteUserGroup(uint(1), uint(2)).Return(nil)
	userRepo.EXPECT().GetUsernameByID(uint(1)).Return("admin", nil)
	projectRepo.EXPECT().ListProjectsByGroup(uint(2)).Return([]models.Project{{PID: 100}}, nil)

	err := svc.DeleteUserGroup(ctx, 1, 2)

	assert.NoError(t, err)
}

func TestDeleteUserGroup_Fail_DeleteRepo(t *testing.T) {
	svc, ugRepo, _, _, _, ctx := setupUserGroupMocks(t)

	oldUG := models.UserGroupView{UID: 1, GID: 2}
	ugRepo.EXPECT().GetUserGroup(uint(1), uint(2)).Return(oldUG, nil)
	ugRepo.EXPECT().DeleteUserGroup(uint(1), uint(2)).Return(errors.New("delete fail"))

	err := svc.DeleteUserGroup(ctx, 1, 2)

	assert.Error(t, err)
}

// ---------- AllocateGroupResource ----------
func TestAllocateGroupResource_Success(t *testing.T) {
	svc, _, _, projectRepo, _, _ := setupUserGroupMocks(t)

	projectRepo.EXPECT().ListProjectsByGroup(uint(1)).Return([]models.Project{{PID: 100}}, nil)

	err := svc.AllocateGroupResource(1, "admin")

	assert.NoError(t, err)
}

func TestAllocateGroupResource_Fail_ListProjects(t *testing.T) {
	svc, _, _, projectRepo, _, _ := setupUserGroupMocks(t)

	projectRepo.EXPECT().ListProjectsByGroup(uint(1)).Return(nil, errors.New("db fail"))

	err := svc.AllocateGroupResource(1, "admin")

	assert.Error(t, err)
}

// ---------- RemoveGroupResource ----------
func TestRemoveGroupResource_Success(t *testing.T) {
	svc, _, _, projectRepo, _, _ := setupUserGroupMocks(t)

	projectRepo.EXPECT().ListProjectsByGroup(uint(1)).Return([]models.Project{{PID: 100}}, nil)

	err := svc.RemoveGroupResource(1, "admin")

	assert.NoError(t, err)
}

// ---------- Formatter ----------
func TestFormatByUID(t *testing.T) {
	svc, _, _, _, _, _ := setupUserGroupMocks(t)

	records := []models.UserGroupView{
		{UID: 1, Username: "alice", GID: 10, GroupName: "g10", Role: "user"},
		{UID: 1, Username: "alice", GID: 11, GroupName: "g11", Role: "admin"},
		{UID: 2, Username: "bob", GID: 10, GroupName: "g10", Role: "user"},
	}

	res := svc.FormatByUID(records)

	assert.Len(t, res, 2)
	assert.Equal(t, "alice", res[1].Username)
	assert.Len(t, res[1].Groups, 2)
	assert.Equal(t, "bob", res[2].Username)
}

func TestFormatByGID(t *testing.T) {
	svc, _, _, _, _, _ := setupUserGroupMocks(t)

	records := []models.UserGroupView{
		{UID: 1, Username: "alice", GID: 10, GroupName: "g10", Role: "user"},
		{UID: 2, Username: "bob", GID: 10, GroupName: "g10", Role: "admin"},
	}

	res := svc.FormatByGID(records)

	assert.Len(t, res, 1)
	assert.Equal(t, "g10", res[10].GroupName)
	assert.Len(t, res[10].Users, 2)
}

func TestFormatByUID_Empty(t *testing.T) {
	svc, _, _, _, _, _ := setupUserGroupMocks(t)
	res := svc.FormatByUID([]models.UserGroupView{})
	assert.Len(t, res, 0)
}

func TestFormatByGID_Empty(t *testing.T) {
	svc, _, _, _, _, _ := setupUserGroupMocks(t)
	res := svc.FormatByGID([]models.UserGroupView{})
	assert.Len(t, res, 0)
}
