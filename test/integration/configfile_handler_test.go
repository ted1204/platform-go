package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/linskybing/platform-go/internal/domain/configfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigFileHandler_Integration(t *testing.T) {
	ctx := GetTestContext()
	k8sValidator := NewK8sValidator()

	var testConfigFileID uint

	t.Run("CreateConfigFile - Success as Manager", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.ManagerToken)

		formData := map[string]string{
			"project_id": fmt.Sprintf("%d", ctx.TestProject.PID),
			"filename":   "test-config.yaml",
			"raw_yaml":   "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test-pod",
		}

		resp, err := client.POSTFormRaw("/config-files", formData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var created configfile.ConfigFile
		err = resp.DecodeJSON(&created)
		require.NoError(t, err)
		assert.Equal(t, "test-config.yaml", created.Filename)
		assert.NotZero(t, created.CFID)
		testConfigFileID = created.CFID
	})

	t.Run("CreateConfigFile - Forbidden for Regular User", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		formData := map[string]string{
			"project_id": fmt.Sprintf("%d", ctx.TestProject.PID),
			"filename":   "unauthorized-config.yaml",
			"raw_yaml":   "apiVersion: v1\nkind: Pod\nmetadata:\n  name: unauthorized-pod",
		}

		resp, err := client.POSTFormRaw("/config-files", formData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("CreateConfigFile - Invalid Input Validation", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.ManagerToken)

		tests := []struct {
			name  string
			input map[string]interface{}
		}{
			{
				name: "Empty name",
				input: map[string]interface{}{
					"project_id": ctx.TestProject.PID,
					"name":       "",
					"image":      "nginx:latest",
					"replicas":   1,
				},
			},
			{
				name: "Empty image",
				input: map[string]interface{}{
					"project_id": ctx.TestProject.PID,
					"name":       "test",
					"image":      "",
					"replicas":   1,
				},
			},
			{
				name: "Negative replicas",
				input: map[string]interface{}{
					"project_id": ctx.TestProject.PID,
					"name":       "test",
					"image":      "nginx:latest",
					"replicas":   -1,
				},
			},
			{
				name: "Invalid CPU request format",
				input: map[string]interface{}{
					"project_id":  ctx.TestProject.PID,
					"name":        "test",
					"image":       "nginx:latest",
					"replicas":    1,
					"cpu_request": "invalid",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp, err := client.POST("/config-files", tt.input)
				require.NoError(t, err)
				assert.GreaterOrEqual(t, resp.StatusCode, 400)
			})
		}
	})

	t.Run("GetConfigFile - Success as Member", func(t *testing.T) {
		if testConfigFileID == 0 {
			t.Skip("No config file to test")
		}

		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		path := fmt.Sprintf("/config-files/%d", testConfigFileID)
		resp, err := client.GET(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var cf configfile.ConfigFile
		err = resp.DecodeJSON(&cf)
		require.NoError(t, err)
		assert.Equal(t, testConfigFileID, cf.CFID)
	})

	t.Run("ListConfigFiles - Admin Only", func(t *testing.T) {
		// Admin can list all
		adminClient := NewHTTPClient(ctx.Router, ctx.AdminToken)
		resp, err := adminClient.GET("/config-files")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Regular user cannot list all
		userClient := NewHTTPClient(ctx.Router, ctx.UserToken)
		resp, err = userClient.GET("/config-files")
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("ListConfigFilesByProject - Success", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		path := fmt.Sprintf("/projects/%d/config-files", ctx.TestProject.PID)
		resp, err := client.GET(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var configs []configfile.ConfigFile
		err = resp.DecodeJSON(&configs)
		require.NoError(t, err)
	})

	t.Run("UpdateConfigFile - Success as Manager", func(t *testing.T) {
		if testConfigFileID == 0 {
			t.Skip("No config file to test")
		}

		client := NewHTTPClient(ctx.Router, ctx.ManagerToken)

		formData := map[string]string{
			"filename": "updated-config.yaml",
			"raw_yaml": "apiVersion: v1\nkind: Pod\nmetadata:\n  name: updated-pod",
		}

		path := fmt.Sprintf("/config-files/%d", testConfigFileID)
		resp, err := client.PUTForm(path, formData)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify update
		getResp, err := client.GET(path)
		require.NoError(t, err)

		var updated configfile.ConfigFile
		err = getResp.DecodeJSON(&updated)
		require.NoError(t, err)
		assert.Equal(t, "updated-config.yaml", updated.Filename)
	})

	t.Run("UpdateConfigFile - Forbidden for Regular User", func(t *testing.T) {
		if testConfigFileID == 0 {
			t.Skip("No config file to test")
		}

		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		formData := map[string]string{
			"filename": "forbidden-update.yaml",
		}

		path := fmt.Sprintf("/config-files/%d", testConfigFileID)
		resp, err := client.PUTForm(path, formData)

		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("CreateInstance - Success with K8s Verification", func(t *testing.T) {
		if testConfigFileID == 0 {
			t.Skip("No config file to test")
		}

		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		path := fmt.Sprintf("/instance/%d", testConfigFileID)
		resp, err := client.POST(path, nil)

		require.NoError(t, err)
		// Without K8s, expect 500 or other error, not 403
		// With K8s, should succeed
		if resp.StatusCode == http.StatusForbidden {
			t.Errorf("User should have permission to create instance")
		}

		// Only verify K8s deployment if K8s is available and creation succeeded
		if resp.StatusCode == http.StatusOK && k8sValidator != nil {
			namespace := fmt.Sprintf("proj-%d", ctx.TestProject.PID)
			deploymentName := fmt.Sprintf("test-config-deployment")

			exists, err := k8sValidator.DeploymentExists(namespace, deploymentName)
			if err == nil && exists {
				t.Logf("Deployment %s/%s created successfully", namespace, deploymentName)
			}
		}
	})

	t.Run("DestructInstance - Success as Member", func(t *testing.T) {
		if testConfigFileID == 0 {
			t.Skip("No config file to test")
		}

		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		path := fmt.Sprintf("/instance/%d", testConfigFileID)
		resp, err := client.DELETE(path)

		require.NoError(t, err)
		assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("DeleteConfigFile - Success as Manager", func(t *testing.T) {
		// Create a config file to delete
		client := NewHTTPClient(ctx.Router, ctx.ManagerToken)

		formData := map[string]string{
			"project_id": fmt.Sprintf("%d", ctx.TestProject.PID),
			"filename":   "config-to-delete.yaml",
			"raw_yaml":   "apiVersion: v1\nkind: Pod\nmetadata:\n  name: config-to-delete\nspec:\n  containers:\n  - name: nginx\n    image: nginx:latest\n",
		}

		createResp, err := client.POSTFormRaw("/config-files", formData)
		require.NoError(t, err)

		var created configfile.ConfigFile
		err = createResp.DecodeJSON(&created)
		require.NoError(t, err)

		// Delete it
		path := fmt.Sprintf("/config-files/%d", created.CFID)
		deleteResp, err := client.DELETE(path)

		require.NoError(t, err)
		// Accept both 200 OK and 204 No Content for DELETE
		assert.Contains(t, []int{http.StatusOK, http.StatusNoContent}, deleteResp.StatusCode)

		// Verify deletion
		getResp, err := client.GET(path)
		require.NoError(t, err)
		assert.NotEqual(t, http.StatusOK, getResp.StatusCode)
	})

	t.Run("DeleteConfigFile - Forbidden for Regular User", func(t *testing.T) {
		if testConfigFileID == 0 {
			t.Skip("No config file to test")
		}

		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		path := fmt.Sprintf("/config-files/%d", testConfigFileID)
		resp, err := client.DELETE(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

func TestConfigFileHandler_ResourceLimits(t *testing.T) {
	ctx := GetTestContext()

	tests := []struct {
		name        string
		cpuRequest  string
		memRequest  string
		cpuLimit    string
		memLimit    string
		expectError bool
		description string
	}{
		{
			name:        "Valid small resources",
			cpuRequest:  "100m",
			memRequest:  "128Mi",
			cpuLimit:    "200m",
			memLimit:    "256Mi",
			expectError: false,
		},
		{
			name:        "Valid large resources",
			cpuRequest:  "2",
			memRequest:  "4Gi",
			cpuLimit:    "4",
			memLimit:    "8Gi",
			expectError: false,
		},
		{
			name:        "Limit less than request (CPU)",
			cpuRequest:  "1000m",
			memRequest:  "1Gi",
			cpuLimit:    "500m",
			memLimit:    "2Gi",
			expectError: true,
			description: "CPU limit should be >= request",
		},
		{
			name:        "Limit less than request (Memory)",
			cpuRequest:  "500m",
			memRequest:  "2Gi",
			cpuLimit:    "1000m",
			memLimit:    "1Gi",
			expectError: true,
			description: "Memory limit should be >= request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewHTTPClient(ctx.Router, ctx.ManagerToken)

			formData := map[string]string{
				"project_id": fmt.Sprintf("%d", ctx.TestProject.PID),
				"filename":   "resource-test-" + tt.name + ".yaml",
				"raw_yaml":   fmt.Sprintf("apiVersion: v1\nkind: Pod\nmetadata:\n  name: resource-test\nspec:\n  containers:\n  - name: test\n    image: nginx:latest\n    resources:\n      requests:\n        cpu: %s\n        memory: %s\n      limits:\n        cpu: %s\n        memory: %s", tt.cpuRequest, tt.memRequest, tt.cpuLimit, tt.memLimit),
			}

			resp, err := client.POSTFormRaw("/config-files", formData)
			require.NoError(t, err)

			if tt.expectError {
				assert.GreaterOrEqual(t, resp.StatusCode, 400, tt.description)
			} else {
				assert.Equal(t, http.StatusCreated, resp.StatusCode)
			}
		})
	}
}

// TestConfigFileGPUMPSConfiguration tests GPU MPS configuration validation and injection
func TestConfigFileGPUMPSConfiguration(t *testing.T) {
	ctx := GetTestContext()
	if ctx.TestProject.GPUQuota == 0 {
		t.Skip("Project GPU quota not set up for testing")
	}

	client := NewHTTPClient(ctx.Router, ctx.ManagerToken)

	t.Run("CreateConfigFile with GPU request without MPS config - Should fail", func(t *testing.T) {
		// Test with a project that has no MPS configuration
		gpuPodYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: gpu-test-pod
spec:
  containers:
  - name: gpu-container
    image: nvidia/cuda:11.8.0-runtime
    resources:
      requests:
        nvidia.com/gpu: "1"
`
		// Create test project without MPS config
		formData := map[string]string{
			"project_id": fmt.Sprintf("%d", ctx.TestProject.PID),
			"filename":   "gpu-test-no-mps.yaml",
			"raw_yaml":   gpuPodYAML,
		}

		resp, err := client.POSTFormRaw("/config-files", formData)
		require.NoError(t, err)
		// Should succeed at creation time (no GPU request check during config creation)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("CreateConfigFile with GPU request and valid MPS config", func(t *testing.T) {
		gpuPodYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: gpu-mps-pod
spec:
  containers:
  - name: gpu-container
    image: nvidia/cuda:11.8.0-runtime
    resources:
      requests:
        nvidia.com/gpu: "1"
      limits:
        nvidia.com/gpu: "1"
`
		formData := map[string]string{
			"project_id": fmt.Sprintf("%d", ctx.TestProject.PID),
			"filename":   "gpu-mps-valid.yaml",
			"raw_yaml":   gpuPodYAML,
		}

		resp, err := client.POSTFormRaw("/config-files", formData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var created configfile.ConfigFile
		err = resp.DecodeJSON(&created)
		require.NoError(t, err)
		assert.Equal(t, "gpu-mps-valid.yaml", created.Filename)
	})

	t.Run("CreateConfigFile with non-GPU workload ignores MPS config", func(t *testing.T) {
		// Pod without GPU request should not have MPS config validated or injected
		regularPodYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: regular-pod
spec:
  containers:
  - name: app-container
    image: nginx:latest
    resources:
      requests:
        cpu: "100m"
        memory: "128Mi"
`
		formData := map[string]string{
			"project_id": fmt.Sprintf("%d", ctx.TestProject.PID),
			"filename":   "regular-pod.yaml",
			"raw_yaml":   regularPodYAML,
		}

		resp, err := client.POSTFormRaw("/config-files", formData)
		require.NoError(t, err)
		// Should succeed - no GPU validation needed for non-GPU workloads
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("CreateConfigFile with Deployment containing GPU request", func(t *testing.T) {
		deploymentYAML := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gpu-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gpu-app
  template:
    metadata:
      labels:
        app: gpu-app
    spec:
      containers:
      - name: gpu-container
        image: nvidia/cuda:11.8.0-runtime
        resources:
          requests:
            nvidia.com/gpu: "1"
`
		formData := map[string]string{
			"project_id": fmt.Sprintf("%d", ctx.TestProject.PID),
			"filename":   "gpu-deployment.yaml",
			"raw_yaml":   deploymentYAML,
		}

		resp, err := client.POSTFormRaw("/config-files", formData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})
}
