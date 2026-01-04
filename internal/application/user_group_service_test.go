package application

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/linskybing/platform-go/internal/domain/group"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/internal/repository/mock"
	"github.com/linskybing/platform-go/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func setupUserGroupMocks(t *testing.T) (*UserGroupService,
	*mock.MockUserGroupRepo,
	*mock.MockUserRepo,
	*mock.MockProjectRepo,
	*mock.MockGroupRepo,
	*mock.MockAuditRepo,
	*gin.Context) {

	ctrl := gomock.NewController(t)
	t.Cleanup(func() { ctrl.Finish() })

	mockUG := mock.NewMockUserGroupRepo(ctrl)
	mockUser := mock.NewMockUserRepo(ctrl)
	mockProject := mock.NewMockProjectRepo(ctrl)
	mockGroup := mock.NewMockGroupRepo(ctrl)
	mockAudit := mock.NewMockAuditRepo(ctrl)

	repos := &repository.Repos{
		UserGroup: mockUG,
		User:      mockUser,
		Project:   mockProject,
		Group:     mockGroup,
		Audit:     mockAudit,
	}

	service := NewUserGroupService(repos)
	ctx, _ := gin.CreateTestContext(nil)

	// override utils
	utils.LogAuditWithConsole = func(ctx *gin.Context, action, resourceType, resourceID string,
		oldData, newData interface{}, msg string, repos repository.AuditRepo) {
	}
	utils.FormatNamespaceName = func(pid uint, username string) string {
		return fmt.Sprintf("ns-%d-%s", pid, username)
	}
	utils.CreateNamespace = func(ns string) error { return nil }
	utils.CreatePVC = func(ns, name, class, size string) error { return nil }
	utils.DeleteNamespace = func(ns string) error { return nil }

	return service, mockUG, mockUser, mockProject, mockGroup, mockAudit, ctx
}

//
// --- TESTS ---
//

// ---------- CreateUserGroup ----------
func TestCreateUserGroup_Success(t *testing.T) {
	svc, ugRepo, userRepo, projectRepo, _, _, ctx := setupUserGroupMocks(t)

	ug := &group.UserGroup{UID: 1, GID: 2}
	projects := []project.Project{{PID: 100, ProjectName: "p1"}}

	ugRepo.EXPECT().CreateUserGroup(ug).Return(nil)
	userRepo.EXPECT().GetUsernameByID(uint(1)).Return("admin", nil)
	projectRepo.EXPECT().ListProjectsByGroup(uint(2)).Return(projects, nil)

	res, err := svc.CreateUserGroup(ctx, ug)

	assert.NoError(t, err)
	assert.Equal(t, ug, res)
}

func TestCreateUserGroup_Fail_CreateRepo(t *testing.T) {
	svc, ugRepo, _, _, _, _, ctx := setupUserGroupMocks(t)

	ug := &group.UserGroup{UID: 1, GID: 2}
	ugRepo.EXPECT().CreateUserGroup(ug).Return(errors.New("db error"))

	res, err := svc.CreateUserGroup(ctx, ug)

	assert.Error(t, err)
	assert.Nil(t, res)
}

func TestCreateUserGroup_Fail_GetUser(t *testing.T) {
	svc, ugRepo, userRepo, _, _, _, ctx := setupUserGroupMocks(t)

	ug := &group.UserGroup{UID: 1, GID: 2}
	ugRepo.EXPECT().CreateUserGroup(ug).Return(nil)
	userRepo.EXPECT().GetUsernameByID(uint(1)).Return("", errors.New("user not found"))

	res, err := svc.CreateUserGroup(ctx, ug)

	assert.Error(t, err)
	assert.Nil(t, res)
}

func TestUpdateUserGroup_Success(t *testing.T) {
	svc, ugRepo, _, _, groupRepo, _, ctx := setupUserGroupMocks(t)

	oldUG := group.UserGroup{UID: 1, GID: 1, Role: "user"}
	newUG := &group.UserGroup{UID: 1, GID: 1, Role: "admin"}

		groupRepo.EXPECT().GetGroupByID(uint(1)).Return(group.Group{}, nil)
	ugRepo.EXPECT().UpdateUserGroup(newUG).Return(nil)

	res, err := svc.UpdateUserGroup(ctx, newUG, oldUG)
	assert.NoError(t, err)
	assert.Equal(t, newUG, res)
}

func TestUpdateUserGroup_Fail_UpdateRepo(t *testing.T) {
	svc, ugRepo, _, _, groupRepo, _, ctx := setupUserGroupMocks(t)

	oldUG := group.UserGroup{UID: 1, GID: 1}
	newUG := &group.UserGroup{UID: 1, GID: 1}

		groupRepo.EXPECT().GetGroupByID(uint(1)).Return(group.Group{}, nil)
	ugRepo.EXPECT().UpdateUserGroup(newUG).Return(errors.New("update fail"))

	res, err := svc.UpdateUserGroup(ctx, newUG, oldUG)

	assert.Nil(t, res)
	assert.EqualError(t, err, "update fail")
}

// ---------- DeleteUserGroup ----------
func TestDeleteUserGroup_Success(t *testing.T) {
	svc, ugRepo, userRepo, projectRepo, groupRepo, _, ctx := setupUserGroupMocks(t)

	oldUG := group.UserGroup{UID: 1, GID: 2}
	ugRepo.EXPECT().GetUserGroup(uint(1), uint(2)).Return(oldUG, nil)
		groupRepo.EXPECT().GetGroupByID(uint(2)).Return(group.Group{}, nil)
	ugRepo.EXPECT().DeleteUserGroup(uint(1), uint(2)).Return(nil)
	userRepo.EXPECT().GetUsernameByID(uint(1)).Return("admin", nil)
	projectRepo.EXPECT().ListProjectsByGroup(uint(2)).Return([]project.Project{{PID: 100}}, nil)

	err := svc.DeleteUserGroup(ctx, 1, 2)

	assert.NoError(t, err)
}

func TestDeleteUserGroup_Fail_DeleteRepo(t *testing.T) {
	svc, ugRepo, _, _, groupRepo, _, ctx := setupUserGroupMocks(t)

	oldUG := group.UserGroup{UID: 1, GID: 2}
	ugRepo.EXPECT().GetUserGroup(uint(1), uint(2)).Return(oldUG, nil)
	groupRepo.EXPECT().GetGroupByID(uint(2)).Return(group.Group{}, nil)
	ugRepo.EXPECT().DeleteUserGroup(uint(1), uint(2)).Return(errors.New("delete fail"))

	err := svc.DeleteUserGroup(ctx, 1, 2)

	assert.Error(t, err)
}

// ---------- AllocateGroupResource ----------
func TestAllocateGroupResource_Success(t *testing.T) {
	svc, _, _, projectRepo, _, _, _ := setupUserGroupMocks(t)

	projectRepo.EXPECT().ListProjectsByGroup(uint(1)).Return([]project.Project{{PID: 100}}, nil)

	err := svc.AllocateGroupResource(1, "admin")

	assert.NoError(t, err)
}

func TestAllocateGroupResource_Fail_ListProjects(t *testing.T) {
	svc, _, _, projectRepo, _, _, _ := setupUserGroupMocks(t)

	projectRepo.EXPECT().ListProjectsByGroup(uint(1)).Return(nil, errors.New("db fail"))

	err := svc.AllocateGroupResource(1, "admin")

	assert.Error(t, err)
}

// ---------- RemoveGroupResource ----------
func TestRemoveGroupResource_Success(t *testing.T) {
	svc, _, _, projectRepo, _, _, _ := setupUserGroupMocks(t)

	projectRepo.EXPECT().ListProjectsByGroup(uint(1)).Return([]project.Project{{PID: 100}}, nil)

	err := svc.RemoveGroupResource(1, "admin")

	assert.NoError(t, err)
}

// ---------- Formatter ----------
func TestFormatByUID(t *testing.T) {
	svc, _, mockUserRepo, _, mockGroupRepo, _, _ := setupUserGroupMocks(t)

	// Add mock expectations for GetUsernameByID calls
	mockUserRepo.EXPECT().GetUsernameByID(uint(1)).Return("user1", nil)
	mockUserRepo.EXPECT().GetUsernameByID(uint(2)).Return("user2", nil)

	// Add mock expectations for GetGroupByID calls
	mockGroupRepo.EXPECT().GetGroupByID(uint(10)).Return(group.Group{
		GID:       10,
		GroupName: "Group10",
	}, nil).Times(2) // Called twice, once for UID 1 and once for UID 2
	mockGroupRepo.EXPECT().GetGroupByID(uint(11)).Return(group.Group{
		GID:       11,
		GroupName: "Group11",
	}, nil)

	records := []group.UserGroup{
		{UID: 1, GID: 10, Role: "user"},
		{UID: 1, GID: 11, Role: "admin"},
		{UID: 2, GID: 10, Role: "user"},
	}

	res := svc.FormatByUID(records)

	assert.Len(t, res, 2)
	assert.NotNil(t, res[1])
	assert.NotNil(t, res[2])

	// Verify UID 1 has 2 groups
	userData1 := res[1]
	assert.Equal(t, uint(1), userData1["UID"])
	assert.Equal(t, "user1", userData1["UserName"])
	groups1 := userData1["Groups"].([]map[string]interface{})
	assert.Len(t, groups1, 2)

	// Verify UID 2 has 1 group
	userData2 := res[2]
	assert.Equal(t, uint(2), userData2["UID"])
	assert.Equal(t, "user2", userData2["UserName"])
	groups2 := userData2["Groups"].([]map[string]interface{})
	assert.Len(t, groups2, 1)
}

func TestFormatByGID(t *testing.T) {
	svc, _, mockUserRepo, _, mockGroupRepo, _, _ := setupUserGroupMocks(t)
	// Add mock expectations for GetUsernameByID calls
	mockUserRepo.EXPECT().GetUsernameByID(uint(1)).Return("user1", nil)
	mockUserRepo.EXPECT().GetUsernameByID(uint(2)).Return("user2", nil)

	// Add mock expectation for GetGroupByID call
	mockGroupRepo.EXPECT().GetGroupByID(uint(10)).Return(group.Group{
		GID:       10,
		GroupName: "TestGroup",
	}, nil)

	records := []group.UserGroup{
		{UID: 1, GID: 10, Role: "user"},
		{UID: 2, GID: 10, Role: "admin"},
	}

	res := svc.FormatByGID(records)

	assert.Len(t, res, 1)
	assert.NotNil(t, res[10])

	// Verify the structure contains Users array
	groupData := res[10]
	assert.Equal(t, uint(10), groupData["GID"])
	assert.Equal(t, "TestGroup", groupData["GroupName"])
	assert.NotNil(t, groupData["Users"])

	users := groupData["Users"].([]map[string]interface{})
	assert.Len(t, users, 2)
}

func TestFormatByUID_Empty(t *testing.T) {
	svc, _, _, _, _, _, _ := setupUserGroupMocks(t)
	res := svc.FormatByUID([]group.UserGroup{})
	assert.Len(t, res, 0)
}

func TestFormatByGID_Empty(t *testing.T) {
	svc, _, _, _, _, _, _ := setupUserGroupMocks(t)
	res := svc.FormatByGID([]group.UserGroup{})
	assert.Len(t, res, 0)
}
