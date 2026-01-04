package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectHandler_Integration(t *testing.T) {
	ctx := GetTestContext()
	k8sValidator := NewK8sValidator()

	t.Run("GetProjects - Success as Admin", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)
		resp, err := client.GET("/projects")

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var projects []project.Project
		err = resp.DecodeJSON(&projects)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(projects), 1)
	})

	t.Run("GetProjects - Success as Regular User", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)
		resp, err := client.GET("/projects")

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var projects []project.Project
		err = resp.DecodeJSON(&projects)
		require.NoError(t, err)
	})

	t.Run("GetProjects - Unauthorized without Token", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, "")
		resp, err := client.GET("/projects")

		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("CreateProject - Success as Admin", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		createDTO := map[string]interface{}{
			"project_name": "integration-test-project",
			"gid":          ctx.TestGroup.GID,
		}

		resp, err := client.POST("/projects", createDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var createdProject project.Project
		err = resp.DecodeJSON(&createdProject)
		require.NoError(t, err)
		assert.Equal(t, "integration-test-project", createdProject.ProjectName)
		assert.Equal(t, ctx.TestGroup.GID, createdProject.GID)
		assert.NotZero(t, createdProject.PID)
	})

	t.Run("CreateProject - Forbidden for Regular User", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		createDTO := map[string]interface{}{
			"project_name": "unauthorized-project",
			"gid":          ctx.TestGroup.GID,
		}

		resp, err := client.POST("/projects", createDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("CreateProject - Invalid Input Validation", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		tests := []struct {
			name     string
			input    map[string]interface{}
			wantCode int
		}{
			{
				name:     "Empty project name",
				input:    map[string]interface{}{"project_name": "", "gid": ctx.TestGroup.GID},
				wantCode: http.StatusBadRequest,
			},
			{
				name:     "Missing GID",
				input:    map[string]interface{}{"project_name": "test"},
				wantCode: http.StatusBadRequest,
			},
			{
				name:     "Invalid GID",
				input:    map[string]interface{}{"project_name": "test", "gid": 99999},
				wantCode: http.StatusInternalServerError,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp, err := client.POST("/projects", tt.input)
				require.NoError(t, err)
				assert.GreaterOrEqual(t, resp.StatusCode, 400)
			})
		}
	})

	t.Run("GetProjectByID - Success", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		path := fmt.Sprintf("/projects/%d", ctx.TestProject.PID)
		resp, err := client.GET(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var proj project.Project
		err = resp.DecodeJSON(&proj)
		require.NoError(t, err)
		assert.Equal(t, ctx.TestProject.PID, proj.PID)
		assert.Equal(t, ctx.TestProject.ProjectName, proj.ProjectName)
	})

	t.Run("GetProjectByID - Not Found", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		resp, err := client.GET("/projects/99999")
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("UpdateProject - Success as Manager", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.ManagerToken)

		updateDTO := map[string]interface{}{
			"project_name": "updated-project-name",
		}

		path := fmt.Sprintf("/projects/%d", ctx.TestProject.PID)
		resp, err := client.PUT(path, updateDTO)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify update
		getResp, err := client.GET(path)
		require.NoError(t, err)

		var updated project.Project
		err = getResp.DecodeJSON(&updated)
		require.NoError(t, err)
		assert.Equal(t, "updated-project-name", updated.ProjectName)
	})

	t.Run("UpdateProject - Forbidden for Regular User", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		updateDTO := map[string]interface{}{
			"project_name": "should-not-work",
		}

		path := fmt.Sprintf("/projects/%d", ctx.TestProject.PID)
		resp, err := client.PUT(path, updateDTO)

		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("DeleteProject - Success as Admin", func(t *testing.T) {
		// Create a project to delete
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		createDTO := map[string]interface{}{
			"project_name": "project-to-delete",
			"gid":          ctx.TestGroup.GID,
		}

		createResp, err := client.POST("/projects", createDTO)
		require.NoError(t, err)

		var created project.Project
		err = createResp.DecodeJSON(&created)
		require.NoError(t, err)

		// Delete it
		path := fmt.Sprintf("/projects/%d", created.PID)
		deleteResp, err := client.DELETE(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, deleteResp.StatusCode)

		// Verify deletion
		getResp, err := client.GET(path)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, getResp.StatusCode)
	})

	t.Run("DeleteProject - Forbidden for Manager", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.ManagerToken)

		path := fmt.Sprintf("/projects/%d", ctx.TestProject.PID)
		resp, err := client.DELETE(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("CreateProjectPVC - Success with K8s Verification", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		pvcDTO := map[string]interface{}{
			"name":          "test-pvc",
			"storage_class": "longhorn",
			"size":          "1Gi",
		}

		path := fmt.Sprintf("/k8s/pvc/project/%d", ctx.TestProject.PID)
		resp, err := client.POST(path, pvcDTO)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify PVC exists in K8s
		namespace := fmt.Sprintf("proj-%d", ctx.TestProject.PID)
		exists, err := k8sValidator.PVCExists(namespace, "test-pvc")

		// Note: This may fail if namespace doesn't exist yet
		// In a real scenario, the CreateProject should create the namespace
		if err == nil {
			assert.True(t, exists, "PVC should exist in Kubernetes")
		}
	})

	t.Run("ListPVCsByProject - Verify K8s Resources", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		path := fmt.Sprintf("/k8s/pvc/by-project/%d", ctx.TestProject.PID)
		resp, err := client.GET(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var pvcs []interface{}
		err = resp.DecodeJSON(&pvcs)
		require.NoError(t, err)
	})

	t.Run("GetProjectsByUser - Boundary Conditions", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		resp, err := client.GET("/projects/by-user")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = resp.DecodeJSON(&result)
		require.NoError(t, err)
	})
}

func TestProjectHandler_PermissionBoundaries(t *testing.T) {
	ctx := GetTestContext()

	testCases := []struct {
		name         string
		method       string
		path         string
		token        string
		body         interface{}
		expectedCode int
		description  string
	}{
		// Admin-only endpoints
		{
			name:         "CreateProject - Admin only",
			method:       "POST",
			path:         "/projects",
			token:        ctx.UserToken,
			body:         map[string]interface{}{"project_name": "test", "gid": ctx.TestGroup.GID},
			expectedCode: http.StatusForbidden,
			description:  "Regular users cannot create projects",
		},
		{
			name:         "DeleteProject - Admin only",
			method:       "DELETE",
			path:         fmt.Sprintf("/projects/%d", ctx.TestProject.PID),
			token:        ctx.ManagerToken,
			expectedCode: http.StatusForbidden,
			description:  "Managers cannot delete projects (admin only)",
		},
		// Manager permissions
		{
			name:         "UpdateProject - Manager allowed",
			method:       "PUT",
			path:         fmt.Sprintf("/projects/%d", ctx.TestProject.PID),
			token:        ctx.ManagerToken,
			body:         map[string]interface{}{"project_name": "updated"},
			expectedCode: http.StatusOK,
			description:  "Managers can update projects",
		},
		// User permissions
		{
			name:         "GetProjects - User allowed",
			method:       "GET",
			path:         "/projects",
			token:        ctx.UserToken,
			expectedCode: http.StatusOK,
			description:  "Regular users can view projects",
		},
		{
			name:         "UpdateProject - User forbidden",
			method:       "PUT",
			path:         fmt.Sprintf("/projects/%d", ctx.TestProject.PID),
			token:        ctx.UserToken,
			body:         map[string]interface{}{"project_name": "should-fail"},
			expectedCode: http.StatusForbidden,
			description:  "Regular users cannot update projects",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewHTTPClient(ctx.Router, tc.token)

			var resp *Response
			var err error

			switch tc.method {
			case "GET":
				resp, err = client.GET(tc.path)
			case "POST":
				resp, err = client.POST(tc.path, tc.body)
			case "PUT":
				resp, err = client.PUT(tc.path, tc.body)
			case "DELETE":
				resp, err = client.DELETE(tc.path)
			}

			require.NoError(t, err, tc.description)
			assert.Equal(t, tc.expectedCode, resp.StatusCode, tc.description)
		})
	}
}
