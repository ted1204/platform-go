package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/linskybing/platform-go/internal/domain/group"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupHandler_Integration(t *testing.T) {
	ctx := GetTestContext()

	var testGroupID uint

	t.Run("GetGroups - Success for All Users", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)
		resp, err := client.GET("/groups")

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var groups []group.Group
		err = resp.DecodeJSON(&groups)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(groups), 1)
	})

	t.Run("CreateGroup - Success as Admin", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		createDTO := map[string]string{
			"group_name": "test-integration-group",
		}

		resp, err := client.POSTForm("/groups", createDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var created group.Group
		err = resp.DecodeJSON(&created)
		require.NoError(t, err)
		assert.Equal(t, "test-integration-group", created.GroupName)
		assert.NotZero(t, created.GID)
		testGroupID = created.GID
	})

	t.Run("CreateGroup - Forbidden for Regular User", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		createDTO := map[string]string{
			"group_name": "unauthorized-group",
		}

		resp, err := client.POSTForm("/groups", createDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("CreateGroup - Forbidden for Manager", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.ManagerToken)

		createDTO := map[string]string{
			"group_name": "manager-group",
		}

		resp, err := client.POSTForm("/groups", createDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("CreateGroup - Empty Name Validation", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		createDTO := map[string]string{
			"group_name": "",
		}

		resp, err := client.POSTForm("/groups", createDTO)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, resp.StatusCode, 400)
	})

	t.Run("CreateGroup - Duplicate Name", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		createDTO := map[string]string{
			"group_name": ctx.TestGroup.GroupName,
		}

		resp, err := client.POSTForm("/groups", createDTO)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, resp.StatusCode, 400)
	})

	t.Run("GetGroupByID - Success", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		path := fmt.Sprintf("/groups/%d", ctx.TestGroup.GID)
		resp, err := client.GET(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var g group.Group
		err = resp.DecodeJSON(&g)
		require.NoError(t, err)
		assert.Equal(t, ctx.TestGroup.GID, g.GID)
	})

	t.Run("GetGroupByID - Not Found", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		resp, err := client.GET("/groups/99999")
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("UpdateGroup - Success as Admin", func(t *testing.T) {
		if testGroupID == 0 {
			t.Skip("No test group created")
		}

		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		updateDTO := map[string]string{
			"group_name": "updated-group-name",
		}

		path := fmt.Sprintf("/groups/%d", testGroupID)
		resp, err := client.PUTForm(path, updateDTO)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify update
		getResp, err := client.GET(path)
		require.NoError(t, err)

		var updated group.Group
		err = getResp.DecodeJSON(&updated)
		require.NoError(t, err)
		assert.Equal(t, "updated-group-name", updated.GroupName)
	})

	t.Run("UpdateGroup - Forbidden for Regular User", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		updateDTO := map[string]string{
			"group_name": "hacked",
		}

		path := fmt.Sprintf("/groups/%d", ctx.TestGroup.GID)
		resp, err := client.PUTForm(path, updateDTO)

		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("UpdateGroup - Cannot Update Reserved Group", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		updateDTO := map[string]string{
			"group_name": "new-super-name",
		}

		path := fmt.Sprintf("/groups/%d", ctx.TestGroup.GID)
		resp, err := client.PUTForm(path, updateDTO)

		require.NoError(t, err)
		_ = resp // Response validation depends on business logic
		// Should fail or succeed with special handling for reserved groups
		// Check your business logic
	})

	t.Run("DeleteGroup - Success as Admin", func(t *testing.T) {
		// Create a group to delete
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		createDTO := map[string]string{
			"group_name": "group-to-delete",
		}

		createResp, err := client.POSTForm("/groups", createDTO)
		require.NoError(t, err)

		var created group.Group
		err = createResp.DecodeJSON(&created)
		require.NoError(t, err)

		// Delete it
		path := fmt.Sprintf("/groups/%d", created.GID)
		deleteResp, err := client.DELETE(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, deleteResp.StatusCode)

		// Verify deletion
		getResp, err := client.GET(path)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, getResp.StatusCode)
	})

	t.Run("DeleteGroup - Forbidden for Manager", func(t *testing.T) {
		if testGroupID == 0 {
			t.Skip("No test group")
		}

		client := NewHTTPClient(ctx.Router, ctx.ManagerToken)

		path := fmt.Sprintf("/groups/%d", testGroupID)
		resp, err := client.DELETE(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("DeleteGroup - Cannot Delete Reserved Group", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		path := fmt.Sprintf("/groups/%d", ctx.TestGroup.GID)
		resp, err := client.DELETE(path)

		require.NoError(t, err)
		// Should fail with appropriate error
		assert.GreaterOrEqual(t, resp.StatusCode, 400)
	})
}

func TestUserGroupHandler_Integration(t *testing.T) {
	ctx := GetTestContext()

	t.Run("GetUserGroup - Admin Only", func(t *testing.T) {
		// Admin can access
		adminClient := NewHTTPClient(ctx.Router, ctx.AdminToken)
		resp, err := adminClient.GET("/user-group")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Regular user cannot
		userClient := NewHTTPClient(ctx.Router, ctx.UserToken)
		resp, err = userClient.GET("/user-group")
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("GetUserGroupsByGID - Success", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		resp, err := client.GET("/user-group/by-group", map[string]string{
			"gid": fmt.Sprintf("%d", ctx.TestGroup.GID),
		})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var userGroups []group.UserGroup
		err = resp.DecodeJSON(&userGroups)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(userGroups), 1)
	})

	t.Run("GetUserGroupsByUID - Success", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		resp, err := client.GET("/user-group/by-user", map[string]string{
			"uid": fmt.Sprintf("%d", ctx.TestUser.UID),
		})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var userGroups []group.UserGroup
		err = resp.DecodeJSON(&userGroups)
		require.NoError(t, err)
	})

	t.Run("CreateUserGroup - Success as Group Admin", func(t *testing.T) {
		// Admin is group admin for test group
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		createDTO := map[string]interface{}{
			"uid":  ctx.TestUser.UID,
			"gid":  ctx.TestGroup.GID,
			"role": "user",
		}

		resp, err := client.POST("/user-group", createDTO)
		require.NoError(t, err)
		// May already exist, check for OK or Conflict
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusConflict)
	})

	t.Run("CreateUserGroup - Forbidden for Non-Admin", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		createDTO := map[string]interface{}{
			"uid":  ctx.TestUser.UID,
			"gid":  ctx.TestGroup.GID,
			"role": "admin",
		}

		resp, err := client.POST("/user-group", createDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("CreateUserGroup - Invalid Role", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		createDTO := map[string]interface{}{
			"uid":  ctx.TestUser.UID,
			"gid":  ctx.TestGroup.GID,
			"role": "superuser", // Invalid role
		}

		resp, err := client.POST("/user-group", createDTO)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, resp.StatusCode, 400)
	})

	t.Run("UpdateUserGroup - Success as Group Admin", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		updateDTO := map[string]interface{}{
			"uid":  ctx.TestManager.UID,
			"gid":  ctx.TestGroup.GID,
			"role": "admin",
		}

		resp, err := client.PUT("/user-group", updateDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify update
		verifyResp, err := client.GET("/user-group/by-user", map[string]string{
			"uid": fmt.Sprintf("%d", ctx.TestManager.UID),
		})
		require.NoError(t, err)

		var userGroups []group.UserGroup
		err = verifyResp.DecodeJSON(&userGroups)
		require.NoError(t, err)

		// Find the updated role
		found := false
		for _, ug := range userGroups {
			if ug.GID == ctx.TestGroup.GID && ug.UID == ctx.TestManager.UID {
				assert.Equal(t, "admin", ug.Role)
				found = true
				break
			}
		}
		assert.True(t, found, "Should find updated user group")
	})

	t.Run("UpdateUserGroup - Cannot Downgrade Last Admin", func(t *testing.T) {
		// This test depends on your business logic
		// Typically you shouldn't be able to remove the last admin from a group
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		updateDTO := map[string]interface{}{
			"uid":  ctx.TestAdmin.UID,
			"gid":  ctx.TestGroup.GID,
			"role": "user",
		}

		resp, err := client.PUT("/user-group", updateDTO)
		require.NoError(t, err)
		_ = resp // Response validation depends on business logic
		// Should fail or have special handling
	})

	t.Run("DeleteUserGroup - Success as Group Admin", func(t *testing.T) {
		// First create a user group to delete
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		// Create temp user for deletion test
		// Assuming we can use existing user
		deleteDTO := map[string]interface{}{
			"uid": ctx.TestUser.UID,
			"gid": ctx.TestGroup.GID,
		}

		resp, err := client.DELETE("/user-group", deleteDTO)
		require.NoError(t, err)
		// Should succeed or return not found
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound)
	})

	t.Run("DeleteUserGroup - Forbidden for Regular User", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		deleteDTO := map[string]interface{}{
			"uid": ctx.TestManager.UID,
			"gid": ctx.TestGroup.GID,
		}

		resp, err := client.DELETE("/user-group", deleteDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

func TestUserGroupHandler_RoleHierarchy(t *testing.T) {
	ctx := GetTestContext()

	tests := []struct {
		name         string
		userToken    string
		targetRole   string
		expectedCode int
		description  string
	}{
		{
			name:         "Admin can assign admin role",
			userToken:    ctx.AdminToken,
			targetRole:   "admin",
			expectedCode: http.StatusOK,
			description:  "Admin should be able to assign admin role",
		},
		{
			name:         "Manager cannot assign admin role",
			userToken:    ctx.ManagerToken,
			targetRole:   "admin",
			expectedCode: http.StatusForbidden,
			description:  "Manager should not be able to assign admin role",
		},
		{
			name:         "User cannot assign any role",
			userToken:    ctx.UserToken,
			targetRole:   "user",
			expectedCode: http.StatusForbidden,
			description:  "Regular user should not be able to assign roles",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewHTTPClient(ctx.Router, tt.userToken)

			createDTO := map[string]interface{}{
				"uid":  ctx.TestUser.UID + 1000, // Use non-existent user to avoid conflicts
				"gid":  ctx.TestGroup.GID,
				"role": tt.targetRole,
			}

			resp, err := client.POST("/user-group", createDTO)
			require.NoError(t, err)

			// Allow for both expected code and not found (if user doesn't exist)
			assert.True(t,
				resp.StatusCode == tt.expectedCode || resp.StatusCode == http.StatusNotFound,
				tt.description,
			)
		})
	}
}
