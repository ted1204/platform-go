package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/models"
	"github.com/linskybing/platform-go/response"
	"github.com/stretchr/testify/require"
)

// TestGroupFlow tests the full group workflow including authorization checks.
// Non-admin users should be forbidden from creating groups.
// Admin users should be able to create, list, and get groups.
func TestGroupFlow(t *testing.T) {

	// 1 Login admin and normal user
	adminToken := loginUser(t, "admin", "1234")
	require.NotEmpty(t, adminToken, "admin token should not be empty")

	userToken := loginUser(t, "alice", "123456")
	require.NotEmpty(t, userToken, "user token should not be empty")

	// 2 Normal user attempts to create a group -> expect 403 Forbidden
	form := url.Values{}
	form.Add("group_name", "nonadmin_group")
	form.Add("description", "should fail")

	resp := doRequest(t, "POST", "/groups", userToken, form, http.StatusForbidden)
	t.Logf("Non-admin create group response body: %s", resp.Body.String())

	// 3 Admin creates a group successfully -> expect 201 Created
	groupName := "admin_group"
	description := "created by admin"

	form = url.Values{}
	form.Add("group_name", groupName)
	form.Add("description", description)

	resp = doRequest(t, "POST", "/groups", adminToken, form, http.StatusCreated)
	t.Logf("Admin create group response body: %s", resp.Body.String())

	var newGroup models.Group
	err := json.Unmarshal(resp.Body.Bytes(), &newGroup)
	require.NoError(t, err)
	require.Equal(t, groupName, newGroup.GroupName)
	require.Equal(t, description, newGroup.Description)
	require.NotZero(t, newGroup.GID)
	require.False(t, newGroup.CreatedAt.IsZero(), "CreatedAt should be set")
	require.False(t, newGroup.UpdatedAt.IsZero(), "UpdatedAt should be set")

	// 4 List all groups -> should include the newly created group
	resp = doRequest(t, "GET", "/groups", adminToken, nil, http.StatusOK)

	var groups []models.Group
	err = json.Unmarshal(resp.Body.Bytes(), &groups)
	require.NoError(t, err)

	found := false
	for _, g := range groups {
		if g.GroupName == groupName {
			found = true
			break
		}
	}
	require.True(t, found, "groups should contain the newly created group")

	// 5 Get group by ID -> should return correct group
	resp = doRequest(t, "GET", fmt.Sprintf("/groups/%d", newGroup.GID), adminToken, nil, http.StatusOK)

	var fetchedGroup models.Group
	err = json.Unmarshal(resp.Body.Bytes(), &fetchedGroup)
	require.NoError(t, err)
	require.Equal(t, newGroup.GID, fetchedGroup.GID)
	require.Equal(t, groupName, fetchedGroup.GroupName)
	require.Equal(t, description, fetchedGroup.Description)
	require.False(t, fetchedGroup.CreatedAt.IsZero(), "CreatedAt should be set")
	require.False(t, fetchedGroup.UpdatedAt.IsZero(), "UpdatedAt should be set")
}

func TestGroupUpdateAndDeleteFlow(t *testing.T) {

	// 1 Login admin
	adminToken := loginUser(t, "admin", "1234")
	require.NotEmpty(t, adminToken)

	// 2 Admin creates a group first
	groupForm := url.Values{}
	groupForm.Add("group_name", "test_team")
	groupForm.Add("description", "original description")

	resp := doRequest(t, "POST", "/groups", adminToken, groupForm, http.StatusCreated)

	var createdGroup models.Group
	err := json.Unmarshal(resp.Body.Bytes(), &createdGroup)
	require.NoError(t, err)
	require.Equal(t, "test_team", createdGroup.GroupName)

	// 3 Update group
	updateForm := url.Values{}
	updateForm.Add("group_name", "updated_team")
	updateForm.Add("description", "updated description")

	updateURL := fmt.Sprintf("/groups/%d", createdGroup.GID)
	resp = doRequest(t, "PUT", updateURL, adminToken, updateForm, http.StatusOK)

	var updatedGroup models.Group
	err = json.Unmarshal(resp.Body.Bytes(), &updatedGroup)
	require.NoError(t, err)
	require.Equal(t, createdGroup.GID, updatedGroup.GID)
	require.Equal(t, "updated_team", updatedGroup.GroupName)
	require.Equal(t, "updated description", updatedGroup.Description)

	// 4 Delete group
	deleteURL := fmt.Sprintf("/groups/%d", createdGroup.GID)
	resp = doRequest(t, "DELETE", deleteURL, adminToken, nil, http.StatusOK)

	var msgResp response.MessageResponse
	err = json.Unmarshal(resp.Body.Bytes(), &msgResp)
	require.NoError(t, err)
	require.Equal(t, "Group deleted", msgResp.Message)

	// 5 Verify group no longer exists
	resp = doRequest(t, "GET", fmt.Sprintf("/groups/%d", createdGroup.GID), adminToken, nil, http.StatusNotFound)
}

func TestGroupUpdateAndDeleteFlowWithForbiddenUser(t *testing.T) {

	// 1 Login admin and normal user
	adminToken := loginUser(t, "admin", "1234")
	require.NotEmpty(t, adminToken)

	userToken := loginUser(t, "alice", "123456")
	require.NotEmpty(t, userToken)

	// 2 Admin creates a group
	groupForm := url.Values{}
	groupForm.Add("group_name", "test_team")
	groupForm.Add("description", "original description")

	resp := doRequest(t, "POST", "/groups", userToken, groupForm, http.StatusForbidden)
	resp = doRequest(t, "POST", "/groups", adminToken, groupForm, http.StatusCreated)

	var createdGroup models.Group
	err := json.Unmarshal(resp.Body.Bytes(), &createdGroup)
	require.NoError(t, err)

	// 3 Normal user attempts to delete the group -> expect 403
	deleteURL := fmt.Sprintf("/groups/%d", createdGroup.GID)
	resp = doRequest(t, "DELETE", deleteURL, userToken, nil, http.StatusForbidden)
	var errResp response.ErrorResponse
	err = json.Unmarshal(resp.Body.Bytes(), &errResp)
	require.NoError(t, err)
	require.Equal(t, "admin only", errResp.Error, "normal user should not be able to delete group")

	// 4 Admin can still delete the group successfully
	resp = doRequest(t, "DELETE", deleteURL, adminToken, nil, http.StatusOK)
	var msgResp response.MessageResponse
	err = json.Unmarshal(resp.Body.Bytes(), &msgResp)
	require.NoError(t, err)
	require.Equal(t, "Group deleted", msgResp.Message)
}

// TestUserGroupFlow tests the workflow of creating user-group relations.
// Only admin users are allowed to create user-group relations.
// Non-admin users should get 403 Forbidden.
func TestUserGroupFlow(t *testing.T) {

	// 1 Login admin and normal user
	adminToken := loginUser(t, "admin", "1234")
	require.NotEmpty(t, adminToken, "admin token should not be empty")

	userToken := loginUser(t, "alice", "123456")
	require.NotEmpty(t, userToken, "user token should not be empty")

	// 2 Admin creates a new group first
	groupForm := url.Values{}
	groupForm.Add("group_name", "team_alpha")
	groupForm.Add("description", "created by admin")

	resp := doRequest(t, "POST", "/groups", adminToken, groupForm, http.StatusCreated)
	t.Logf("Admin create group response body: %s", resp.Body.String())

	var newGroup models.Group
	err := json.Unmarshal(resp.Body.Bytes(), &newGroup)
	require.NoError(t, err)
	require.Equal(t, "team_alpha", newGroup.GroupName)

	// 3 Non-admin tries to create a user-group relation -> expect 403 Forbidden
	userGroupForm := url.Values{}
	userGroupForm.Add("u_id", "2") // alice's UID
	userGroupForm.Add("g_id", fmt.Sprintf("%d", newGroup.GID))
	userGroupForm.Add("role", "user")

	resp = doRequest(t, "POST", "/user-group", userToken, userGroupForm, http.StatusForbidden)
	t.Logf("Non-admin create user-group response body: %s", resp.Body.String())

	// 4 Admin creates a user-group relation successfully -> expect 201 Created
	resp = doRequest(t, "POST", "/user-group", adminToken, userGroupForm, http.StatusCreated)
	t.Logf("Admin create user-group response body: %s", resp.Body.String())

	var userGroup models.UserGroup
	err = json.Unmarshal(resp.Body.Bytes(), &userGroup)
	require.NoError(t, err)
	require.Equal(t, 2, int(userGroup.UID))       // Alice UID
	require.Equal(t, newGroup.GID, userGroup.GID) // Group ID
	require.Equal(t, "user", userGroup.Role)      // Role
	require.False(t, userGroup.CreatedAt.IsZero(), "CreatedAt should be set")
	require.False(t, userGroup.UpdatedAt.IsZero(), "UpdatedAt should be set")
}

func TestUserGroup(t *testing.T) {
	adminToken := loginUser(t, "admin", "1234")
	require.NotEmpty(t, adminToken, "admin token should not be empty")

	aliceToken := loginUser(t, "alice", "123456")
	require.NotEmpty(t, aliceToken, "alice token should not be empty")

	user1Token := loginUser(t, "test1", "123456")
	require.NotEmpty(t, user1Token, "test1 token should not be empty")

	user2Token := loginUser(t, "test2", "123456")
	require.NotEmpty(t, user2Token, "test2 token should not be empty")

	groupID := createGroup(t, adminToken, "dev-team")
	require.NotZero(t, groupID, "groupID should not be zero")

	addUserToGroup(t, adminToken, groupID, 2, "manager", http.StatusCreated) //alice
	addUserToGroup(t, aliceToken, groupID, 3, "user", http.StatusForbidden)  // test1
	addUserToGroup(t, adminToken, groupID, 3, "manager", http.StatusCreated) //alice
	addUserToGroup(t, user1Token, groupID, 4, "user", http.StatusForbidden)  // test2
	addUserToGroup(t, aliceToken, groupID, 4, "user", http.StatusForbidden)  // test2
	addUserToGroup(t, adminToken, groupID, 4, "manager", http.StatusCreated) //alice
	removeUserFromGroup(t, user1Token, groupID, 4, http.StatusForbidden)     // remove test2 fail
	removeUserFromGroup(t, aliceToken, groupID, 4, http.StatusForbidden)     // remove test2 fail
	removeUserFromGroup(t, adminToken, groupID, 4, http.StatusNoContent)     // remove test2 successfully
	addUserToGroup(t, user2Token, groupID, 4, "admin", http.StatusForbidden) // test2
}

func TestUpdateUserGroup(t *testing.T) {
	adminToken := loginUser(t, "admin", "1234")
	require.NotEmpty(t, adminToken)

	userToken := loginUser(t, "alice", "123456")
	require.NotEmpty(t, userToken)

	groupForm := url.Values{}
	groupForm.Add("group_name", "dev_team_2")
	groupForm.Add("description", "created by admin")

	resp := doRequest(t, "POST", "/groups", adminToken, groupForm, http.StatusCreated)
	var group models.Group
	err := json.Unmarshal(resp.Body.Bytes(), &group)
	require.NoError(t, err)

	userGroupForm := url.Values{}
	userGroupForm.Add("u_id", "2")
	userGroupForm.Add("g_id", fmt.Sprintf("%d", group.GID))
	userGroupForm.Add("role", "user")

	resp = doRequest(t, "POST", "/user-group", adminToken, userGroupForm, http.StatusCreated)
	var userGroup models.UserGroup
	err = json.Unmarshal(resp.Body.Bytes(), &userGroup)
	require.NoError(t, err)
	require.Equal(t, "user", userGroup.Role)

	updateForm := url.Values{}
	updateForm.Add("u_id", "2")
	updateForm.Add("g_id", fmt.Sprintf("%d", group.GID))
	updateForm.Add("role", "manager")

	resp = doRequest(t, "PUT", "/user-group", adminToken, updateForm, http.StatusOK)
	var updatedUserGroup models.UserGroup
	err = json.Unmarshal(resp.Body.Bytes(), &updatedUserGroup)
	require.NoError(t, err)
	require.Equal(t, "manager", updatedUserGroup.Role)

	updateForm.Set("role", "admin")
	resp = doRequest(t, "PUT", "/user-group", userToken, updateForm, http.StatusForbidden)
	resp = doRequest(t, "PUT", "/user-group", adminToken, updateForm, http.StatusOK)
	removeUserFromGroup(t, userToken, group.GID, 2, http.StatusNoContent)

	updateForm.Set("u_id", "9999")
	resp = doRequest(t, "PUT", "/user-group", userToken, updateForm, http.StatusForbidden)
	resp = doRequest(t, "PUT", "/user-group", adminToken, updateForm, http.StatusNotFound)
}

func TestUserSelfRemovalFromGroup(t *testing.T) {
	adminToken := loginUser(t, "admin", "1234")
	require.NotEmpty(t, adminToken)

	userToken := loginUser(t, "alice", "123456")
	require.NotEmpty(t, userToken)

	// 1 Admin creates a group
	groupForm := url.Values{}
	groupForm.Add("group_name", "dev_team_self")
	groupForm.Add("description", "created by admin")
	resp := doRequest(t, "POST", "/groups", adminToken, groupForm, http.StatusCreated)

	var group models.Group
	err := json.Unmarshal(resp.Body.Bytes(), &group)
	require.NoError(t, err)

	// 2 Admin adds user to group
	userGroupForm := url.Values{}
	userGroupForm.Add("u_id", "2") // Alice UID
	userGroupForm.Add("g_id", fmt.Sprintf("%d", group.GID))
	userGroupForm.Add("role", "admin")
	resp = doRequest(t, "POST", "/user-group", adminToken, userGroupForm, http.StatusCreated)

	var userGroup models.UserGroup
	err = json.Unmarshal(resp.Body.Bytes(), &userGroup)
	require.NoError(t, err)
	require.Equal(t, "admin", userGroup.Role)

	// 4 User removes self from group
	removeUserFromGroup(t, userToken, group.GID, 2, http.StatusNoContent)

	// 5 Verify group has no users
	urlPath := fmt.Sprintf("/user-group/by-group?g_id=%d", group.GID)
	resp = doRequest(t, "GET", urlPath, adminToken, nil, http.StatusOK)

	var groupUsersResp struct {
		Code    int                       `json:"code"`
		Message string                    `json:"message"`
		Data    map[string]dto.GroupUsers `json:"data"`
	}
	err = json.Unmarshal(resp.Body.Bytes(), &groupUsersResp)
	require.NoError(t, err)

	found := false
	for _, g := range groupUsersResp.Data {
		if g.GID == group.GID && len(g.Users) > 0 {
			found = true
			break
		}
	}
	require.False(t, found, "after self-removal, group should have no users")
}
func TestGetUserGroupInfo(t *testing.T) {
	// 1 Login as admin and normal user
	adminToken := loginUser(t, "admin", "1234")
	require.NotEmpty(t, adminToken)
	userToken := loginUser(t, "alice", "123456")
	require.NotEmpty(t, userToken)

	// 2 Admin creates a group
	groupForm := url.Values{}
	groupForm.Add("group_name", "qa_team")
	groupForm.Add("description", "created by admin")
	resp := doRequest(t, "POST", "/groups", adminToken, groupForm, http.StatusCreated)

	var group models.Group
	err := json.Unmarshal(resp.Body.Bytes(), &group)
	require.NoError(t, err)

	// 3 Admin adds Alice to the group
	userGroupForm := url.Values{}
	userGroupForm.Add("u_id", "2") // Alice UID
	userGroupForm.Add("g_id", fmt.Sprintf("%d", group.GID))
	userGroupForm.Add("role", "user")
	resp = doRequest(t, "POST", "/user-group", adminToken, userGroupForm, http.StatusCreated)

	var createdUG models.UserGroupView
	err = json.Unmarshal(resp.Body.Bytes(), &createdUG)
	require.NoError(t, err)

	// 4 Get a single user-group relation
	urlPath := fmt.Sprintf("/user-group?u_id=%d&g_id=%d", createdUG.UID, createdUG.GID)
	resp = doRequest(t, "GET", urlPath, adminToken, nil, http.StatusOK)

	var getResp struct {
		Code    int              `json:"code"`
		Message string           `json:"message"`
		Data    models.UserGroup `json:"data"`
	}
	err = json.Unmarshal(resp.Body.Bytes(), &getResp)
	require.NoError(t, err)

	require.Equal(t, createdUG.UID, getResp.Data.UID)
	require.Equal(t, createdUG.GID, getResp.Data.GID)
	require.Equal(t, createdUG.Role, getResp.Data.Role)

	// 5 Get all users in the group
	urlPath = fmt.Sprintf("/user-group/by-group?g_id=%d", group.GID)
	resp = doRequest(t, "GET", urlPath, adminToken, nil, http.StatusOK)

	var groupUsersResp struct {
		Code    int                       `json:"code"`
		Message string                    `json:"message"`
		Data    map[string]dto.GroupUsers `json:"data"` // slice, not single object
	}
	err = json.Unmarshal(resp.Body.Bytes(), &groupUsersResp)
	require.NoError(t, err)

	found := false
	for _, group := range groupUsersResp.Data {
		if createdUG.GID != group.GID {
			continue
		}
		for _, user := range group.Users {
			if user.UID == createdUG.UID && user.Role == createdUG.Role {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	require.True(t, found, "created user-group relation should be in the group users response")

	// 6 Get all groups for the user
	urlPath = fmt.Sprintf("/user-group/by-user?u_id=%d", createdUG.UID)
	resp = doRequest(t, "GET", urlPath, adminToken, nil, http.StatusOK)

	var userGroupsResp struct {
		Code    int                       `json:"code"`
		Message string                    `json:"message"`
		Data    map[string]dto.UserGroups `json:"data"` // slice, not single object
	}
	err = json.Unmarshal(resp.Body.Bytes(), &userGroupsResp)
	require.NoError(t, err)

	found = false
	for _, user := range userGroupsResp.Data {
		if user.UID != createdUG.UID {
			continue
		}
		for _, group := range user.Groups {
			if group.GID == createdUG.GID && group.Role == createdUG.Role {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	require.True(t, found, "created user-group relation should appear in user's groups")
}

// --- Helper functions ---
func createGroup(t *testing.T, token, name string) uint {
	form := url.Values{}
	form.Set("group_name", name)
	form.Set("description", "this for test")

	resp := doRequest(t, "POST", "/groups", token, form, http.StatusCreated)

	var group models.UserGroup
	err := json.Unmarshal(resp.Body.Bytes(), &group)
	require.NoError(t, err)
	return group.GID
}

func addUserToGroup(t *testing.T, token string, groupID, uID uint, role string, expectStatus int) {
	form := url.Values{}
	form.Set("u_id", strconv.Itoa(int(uID)))
	form.Set("g_id", strconv.Itoa(int(groupID)))
	form.Set("role", role)

	resp := doRequest(t, "POST", "/user-group", token, form, expectStatus)

	if resp.Code != expectStatus {
		t.Errorf("expected %d, got %d, body=%s", expectStatus, resp.Code, resp.Body.String())
	}
}

func removeUserFromGroup(t *testing.T, token string, groupID, uID uint, expectStatus int) {
	path := fmt.Sprintf("/user-group?u_id=%d&g_id=%d", uID, groupID)
	doRequest(t, "DELETE", path, token, nil, expectStatus)
}
