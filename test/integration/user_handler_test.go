package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/linskybing/platform-go/internal/config/db"
	"github.com/linskybing/platform-go/internal/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserHandler_Integration(t *testing.T) {
	ctx := GetTestContext()

	t.Run("GetUsers - Success for All Users", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)
		resp, err := client.GET("/users")

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var users []user.User
		err = resp.DecodeJSON(&users)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(users), 3) // admin, user, manager
	})

	t.Run("GetUsers - Unauthorized without Token", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, "")
		resp, err := client.GET("/users")

		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("GetUserByID - Success", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		path := fmt.Sprintf("/users/%d", ctx.TestUser.UID)
		resp, err := client.GET(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var u user.User
		err = resp.DecodeJSON(&u)
		require.NoError(t, err)
		assert.Equal(t, ctx.TestUser.UID, u.UID)
		assert.Equal(t, ctx.TestUser.Username, u.Username)
	})

	t.Run("GetUserByID - Not Found", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		resp, err := client.GET("/users/99999")
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("UpdateUser - Success Own Profile", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		newEmail := "newemail@test.com"
		updateDTO := map[string]interface{}{
			"email": newEmail,
		}

		path := fmt.Sprintf("/users/%d", ctx.TestUser.UID)
		resp, err := client.PUT(path, updateDTO)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify update
		getResp, err := client.GET(path)
		require.NoError(t, err)

		var updated user.User
		err = getResp.DecodeJSON(&updated)
		require.NoError(t, err)
		require.NotNil(t, updated.Email)
		assert.Equal(t, newEmail, *updated.Email)
	})

	t.Run("UpdateUser - Forbidden Other User", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		updateDTO := map[string]interface{}{
			"email": "hacked@test.com",
		}

		// Try to update admin's profile
		path := fmt.Sprintf("/users/%d", ctx.TestAdmin.UID)
		resp, err := client.PUT(path, updateDTO)

		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("UpdateUser - Admin Can Update Any User", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		updateDTO := map[string]interface{}{
			"email": "admin-updated@test.com",
		}

		path := fmt.Sprintf("/users/%d", ctx.TestUser.UID)
		resp, err := client.PUT(path, updateDTO)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("DeleteUser - Success Own Account", func(t *testing.T) {
		// Create a temp user for deletion testing
		tempEmail := "temp@test.com"
		tempUser := &user.User{
			Username: "temp-user-delete",
			Email:    &tempEmail,
			Password: "password",
		}
		err := db.DB.Create(tempUser).Error
		require.NoError(t, err)

		// Admin deletes the temp user
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		path := fmt.Sprintf("/users/%d", tempUser.UID)
		resp, err := client.DELETE(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	t.Run("DeleteUser - Forbidden for Other Users", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		// Try to delete admin
		path := fmt.Sprintf("/users/%d", ctx.TestAdmin.UID)
		resp, err := client.DELETE(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("ListUsersPaging - Pagination Works", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		resp, err := client.GET("/users/paging", map[string]string{
			"page":      "1",
			"page_size": "2",
		})

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = resp.DecodeJSON(&result)
		require.NoError(t, err)

		// Should have standard response format with data field
		assert.Contains(t, result, "data")
		// Data should be an array of users
		data, ok := result["data"].([]interface{})
		assert.True(t, ok, "data should be an array")
		assert.GreaterOrEqual(t, len(data), 1, "should have at least 1 user")
	})

	t.Run("Register - Success New User", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, "")

		registerDTO := map[string]interface{}{
			"username": "newuser123",
			"email":    "newuser@test.com",
			"password": "securepassword123",
		}

		resp, err := client.POST("/register", registerDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("Register - Duplicate Username", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, "")

		registerDTO := map[string]interface{}{
			"username": ctx.TestUser.Username,
			"email":    "different@test.com",
			"password": "password123",
		}

		resp, err := client.POST("/register", registerDTO)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, resp.StatusCode, 400)
	})

	t.Run("Register - Invalid Email Format", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, "")

		registerDTO := map[string]interface{}{
			"username": "testuser456",
			"email":    "invalid-email",
			"password": "password123",
		}

		resp, err := client.POST("/register", registerDTO)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, resp.StatusCode, 400)
	})

	t.Run("Register - Weak Password", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, "")

		registerDTO := map[string]interface{}{
			"username": "testuser789",
			"email":    "test789@test.com",
			"password": "123", // Too short
		}

		resp, err := client.POST("/register", registerDTO)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, resp.StatusCode, 400)
	})

	t.Run("Login - Success", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, "")

		loginDTO := map[string]interface{}{
			"username": ctx.TestUser.Username,
			"password": "password123",
		}

		resp, err := client.POST("/login", loginDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = resp.DecodeJSON(&result)
		require.NoError(t, err)
		assert.Contains(t, result, "token")
	})

	t.Run("Login - Wrong Password", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, "")

		loginDTO := map[string]interface{}{
			"username": ctx.TestUser.Username,
			"password": "wrongpassword",
		}

		resp, err := client.POST("/login", loginDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("Login - Nonexistent User", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, "")

		loginDTO := map[string]interface{}{
			"username": "nonexistent",
			"password": "password123",
		}

		resp, err := client.POST("/login", loginDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("Logout - Success", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		resp, err := client.POST("/logout", nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestUserHandler_InputValidation(t *testing.T) {
	ctx := GetTestContext()

	tests := []struct {
		name     string
		endpoint string
		method   string
		token    string
		body     map[string]interface{}
		wantCode int
		desc     string
	}{
		{
			name:     "Empty username on register",
			endpoint: "/register",
			method:   "POST",
			token:    "",
			body: map[string]interface{}{
				"username": "",
				"email":    "test@test.com",
				"password": "password123",
			},
			wantCode: 400,
			desc:     "Username cannot be empty",
		},
		{
			name:     "Very long username",
			endpoint: "/register",
			method:   "POST",
			token:    "",
			body: map[string]interface{}{
				"username": string(make([]byte, 1000)),
				"email":    "test@test.com",
				"password": "password123",
			},
			wantCode: 400,
			desc:     "Should reject excessively long usernames",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewHTTPClient(ctx.Router, tt.token)

			var resp *Response
			var err error

			switch tt.method {
			case "POST":
				resp, err = client.POST(tt.endpoint, tt.body)
			case "PUT":
				resp, err = client.PUT(tt.endpoint, tt.body)
			}

			require.NoError(t, err)
			assert.GreaterOrEqual(t, resp.StatusCode, tt.wantCode, tt.desc)
		})
	}
}
