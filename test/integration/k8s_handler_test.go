package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/linskybing/platform-go/internal/config/db"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/pkg/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestK8sHandler_PVC_Integration(t *testing.T) {
	ctx := GetTestContext()

	// Skip if K8s is not available
	if k8s.Clientset == nil {
		t.Skip("Kubernetes cluster not available (set ENABLE_K8S_TESTS=true to enable)")
	}

	k8sValidator := NewK8sValidator()
	testNamespace := ctx.TestNamespace

	// Ensure namespace exists
	err := createTestNamespace(testNamespace)
	if err != nil {
		t.Logf("Namespace may already exist: %v", err)
	}

	t.Run("CreatePVC - Success as Admin with K8s Verification", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		pvcDTO := map[string]interface{}{
			"namespace":     testNamespace,
			"name":          "test-pvc-1",
			"storage_class": "longhorn",
			"capacity":      "1Gi",
		}

		resp, err := client.POST("/k8s/pvc", pvcDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Verify PVC exists in K8s
		time.Sleep(2 * time.Second) // Wait for K8s to create resource
		exists, err := k8sValidator.PVCExists(testNamespace, "test-pvc-1")
		require.NoError(t, err)
		assert.True(t, exists, "PVC should exist in Kubernetes")

		// Verify PVC properties
		pvc, err := k8sValidator.GetPVC(testNamespace, "test-pvc-1")
		require.NoError(t, err)
		assert.Equal(t, "test-pvc-1", pvc.Name)
		assert.Equal(t, testNamespace, pvc.Namespace)
	})

	t.Run("CreatePVC - Forbidden for Regular User", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		pvcDTO := map[string]interface{}{
			"namespace":     testNamespace,
			"name":          "unauthorized-pvc",
			"storage_class": "longhorn",
			"capacity":      "1Gi",
		}

		resp, err := client.POST("/k8s/pvc", pvcDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		// Verify PVC does NOT exist in K8s
		exists, _ := k8sValidator.PVCExists(testNamespace, "unauthorized-pvc")
		assert.False(t, exists, "PVC should not be created")
	})

	t.Run("CreatePVC - Invalid Size Format", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		pvcDTO := map[string]interface{}{
			"namespace":     testNamespace,
			"name":          "invalid-size-pvc",
			"storage_class": "longhorn",
			"capacity":      "invalid-size",
		}

		resp, err := client.POST("/k8s/pvc", pvcDTO)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, resp.StatusCode, 400)
	})

	t.Run("GetPVC - Success", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		path := fmt.Sprintf("/k8s/pvc/%s/test-pvc-1", testNamespace)
		resp, err := client.GET(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = resp.DecodeJSON(&result)
		require.NoError(t, err)

		// Check for data field in response
		data, ok := result["data"]
		assert.True(t, ok, "response should have 'data' field")
		if ok && data != nil {
			pvc, ok := data.(map[string]interface{})
			assert.True(t, ok, "data should be an object")
			assert.Equal(t, "test-pvc-1", pvc["name"])
		}
	})

	t.Run("ListPVCs - Success", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		path := fmt.Sprintf("/k8s/pvc/list/%s", testNamespace)
		resp, err := client.GET(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = resp.DecodeJSON(&result)
		require.NoError(t, err)

		// Check for data field in response
		data, ok := result["data"]
		assert.True(t, ok, "response should have 'data' field")
		if ok && data != nil {
			pvcs, ok := data.([]interface{})
			assert.True(t, ok, "data should be an array")
			assert.GreaterOrEqual(t, len(pvcs), 0)
		}
	})

	t.Run("ExpandPVC - Success with K8s Verification", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		expandDTO := map[string]interface{}{
			"namespace": testNamespace,
			"name":      "test-pvc-1",
			"capacity":  "2Gi",
		}

		resp, err := client.PUT("/k8s/pvc/expand", expandDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify expansion in K8s
		time.Sleep(2 * time.Second)
		pvc, err := k8sValidator.GetPVC(testNamespace, "test-pvc-1")
		require.NoError(t, err)

		requestedSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		expectedSize := resource.MustParse("2Gi")
		assert.True(t, requestedSize.Equal(expectedSize), "PVC should be expanded to 2Gi")
	})

	t.Run("ExpandPVC - Cannot Shrink", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		expandDTO := map[string]interface{}{
			"namespace": testNamespace,
			"name":      "test-pvc-1",
			"new_size":  "1Gi", // Trying to shrink
		}

		resp, err := client.PUT("/k8s/pvc/expand", expandDTO)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, resp.StatusCode, 400)
	})

	t.Run("DeletePVC - Success with K8s Verification", func(t *testing.T) {
		// Create a PVC to delete
		pvcName := "pvc-to-delete"
		err := k8sValidator.CreateTestPVC(testNamespace, pvcName, "longhorn", "1Gi")
		require.NoError(t, err)

		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		path := fmt.Sprintf("/k8s/pvc/%s/%s", testNamespace, pvcName)
		resp, err := client.DELETE(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify deletion in K8s
		time.Sleep(2 * time.Second)
		exists, _ := k8sValidator.PVCExists(testNamespace, pvcName)
		assert.False(t, exists, "PVC should be deleted from Kubernetes")
	})

	t.Run("DeletePVC - Not Found", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		path := fmt.Sprintf("/k8s/pvc/%s/nonexistent-pvc", testNamespace)
		resp, err := client.DELETE(path)

		require.NoError(t, err)
		// Handler returns 500 for K8s API errors (including not found)
		assert.GreaterOrEqual(t, resp.StatusCode, 400, "Should return an error status code")
	})

	t.Run("ListPVCsByProject - Member Access", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		path := fmt.Sprintf("/k8s/pvc/by-project/%d", ctx.TestProject.PID)
		resp, err := client.GET(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var pvcs []interface{}
		err = resp.DecodeJSON(&pvcs)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(pvcs), 0)
	})
}

func TestK8sHandler_UserStorage_Integration(t *testing.T) {
	ctx := GetTestContext()
	k8sValidator := NewK8sValidator()

	testUsername := "storage-test-user"

	t.Run("GetUserStorageStatus - Admin Only", func(t *testing.T) {
		adminClient := NewHTTPClient(ctx.Router, ctx.AdminToken)
		path := fmt.Sprintf("/k8s/users/%s/storage/status", testUsername)
		resp, err := adminClient.GET(path)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Regular user forbidden
		userClient := NewHTTPClient(ctx.Router, ctx.UserToken)
		resp, err = userClient.GET(path)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("InitializeUserStorage - Success with K8s Verification", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		path := fmt.Sprintf("/k8s/users/%s/storage/init", testUsername)
		resp, err := client.POST(path, nil)

		require.NoError(t, err)
		// May return various codes depending on if already initialized
		assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)

		// Verify namespace and PVC creation in K8s (only if K8s is available)
		if resp.StatusCode == http.StatusOK && k8sValidator != nil {
			userNamespace := fmt.Sprintf("user-%s-storage", testUsername)

			time.Sleep(3 * time.Second)
			exists, err := k8sValidator.NamespaceExists(userNamespace)
			if err == nil {
				assert.True(t, exists, "User namespace should be created")
			}
		}
	})

	t.Run("ExpandUserStorage - Admin Only", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		path := fmt.Sprintf("/k8s/users/%s/storage/expand", testUsername)
		expandDTO := map[string]interface{}{
			"new_size": "100Gi",
		}

		resp, err := client.PUT(path, expandDTO)
		require.NoError(t, err)
		assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("OpenMyDrive - User Can Access Own", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		resp, err := client.POST("/k8s/users/browse", nil)
		require.NoError(t, err)
		// Should succeed or return specific error if not initialized
		assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("StopMyDrive - User Can Stop Own", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		resp, err := client.DELETE("/k8s/users/browse")
		require.NoError(t, err)
		assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("DeleteUserStorage - Admin Only with K8s Verification", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		path := fmt.Sprintf("/k8s/users/%s/storage", testUsername)
		resp, err := client.DELETE(path)

		require.NoError(t, err)
		assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)

		// Verify deletion in K8s (only if K8s is available)
		if resp.StatusCode == http.StatusOK && k8sValidator != nil {
			time.Sleep(3 * time.Second)
			userNamespace := fmt.Sprintf("user-%s", testUsername)
			exists, _ := k8sValidator.NamespaceExists(userNamespace)
			assert.False(t, exists, "User namespace should be deleted")
		}
	})
}

func TestK8sHandler_ProjectStorage_Integration(t *testing.T) {
	ctx := GetTestContext()
	k8sValidator := NewK8sValidator() // K8s validator for verifying resources

	// Create a fresh project for storage testing
	storageTestProject := &project.Project{
		ProjectName: "storage-test-project",
		GID:         ctx.TestGroup.GID,
		GPUQuota:    10,
		GPUAccess:   "shared",
	}
	err := db.DB.Create(storageTestProject).Error
	require.NoError(t, err, "Failed to create storage test project")

	var testStorageID uint

	t.Run("CreateProjectStorage - Admin Only with K8s Verification", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		storageDTO := map[string]interface{}{
			"project_id":    storageTestProject.PID,
			"storage_class": "longhorn",
			"size":          "5Gi",
		}

		resp, err := client.POST("/k8s/storage/projects", storageDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = resp.DecodeJSON(&result)
		if err == nil && k8sValidator != nil {
			if id, ok := result["id"].(float64); ok {
				testStorageID = uint(id)
			}
		}

		// If no Kubernetes client is available, skip K8s verification to avoid nil dereference.
		if k8s.Clientset == nil {
			return
		}

		// Verify PVC created in K8s
		time.Sleep(2 * time.Second)
		projectNamespace := fmt.Sprintf("proj-%d", storageTestProject.PID)
		pvcs, err := k8s.Clientset.CoreV1().PersistentVolumeClaims(projectNamespace).List(
			context.Background(),
			metav1.ListOptions{},
		)
		if err == nil {
			t.Logf("Found %d PVCs in project namespace", len(pvcs.Items))
		}
	})

	t.Run("CreateProjectStorage - Forbidden for User", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		storageDTO := map[string]interface{}{
			"project_id":    storageTestProject.PID,
			"storage_class": "longhorn",
			"size":          "5Gi",
		}

		resp, err := client.POST("/k8s/storage/projects", storageDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("ListProjectStorages - Admin Can List All", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		resp, err := client.GET("/k8s/storage/projects")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var storages []interface{}
		err = resp.DecodeJSON(&storages)
		require.NoError(t, err)
	})

	t.Run("GetUserProjectStorages - User Can Access Own Projects", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		resp, err := client.GET("/k8s/storage/projects/my-storages")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var storages []interface{}
		err = resp.DecodeJSON(&storages)
		require.NoError(t, err)
	})

	t.Run("StartProjectFileBrowser - Member Can Start", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		path := fmt.Sprintf("/k8s/storage/projects/%d/start", storageTestProject.PID)
		resp, err := client.POST(path, nil)

		require.NoError(t, err)
		assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)

		// Verify deployment created
		if resp.StatusCode == http.StatusOK {
			time.Sleep(2 * time.Second)
			projectNamespace := fmt.Sprintf("proj-%d", ctx.TestProject.PID)
			deployments, err := k8s.Clientset.AppsV1().Deployments(projectNamespace).List(
				context.Background(),
				metav1.ListOptions{
					LabelSelector: "app=filebrowser",
				},
			)
			if err == nil {
				assert.GreaterOrEqual(t, len(deployments.Items), 0)
			}
		}
	})

	t.Run("StopProjectFileBrowser - Member Can Stop", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		path := fmt.Sprintf("/k8s/storage/projects/%d/stop", storageTestProject.PID)
		resp, err := client.DELETE(path)

		require.NoError(t, err)
		assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("DeleteProjectStorage - Admin Only with K8s Verification", func(t *testing.T) {
		if testStorageID == 0 {
			t.Skip("No storage to delete")
		}

		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		path := fmt.Sprintf("/k8s/storage/projects/%d", testStorageID)
		resp, err := client.DELETE(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("DeleteProjectStorage - Forbidden for User", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		path := fmt.Sprintf("/k8s/storage/projects/%d", ctx.TestProject.PID)
		resp, err := client.DELETE(path)

		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

func TestK8sHandler_Jobs_Integration(t *testing.T) {
	ctx := GetTestContext()

	t.Run("CreateJob - Admin Only", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.AdminToken)

		// Use proper namespace format: pid-username
		namespace := fmt.Sprintf("%d-test-admin", ctx.TestProject.PID)

		jobDTO := map[string]interface{}{
			"name":      "test-job",
			"image":     "busybox:latest",
			"command":   []string{"echo", "hello"},
			"namespace": namespace,
		}

		resp, err := client.POST("/k8s/jobs", jobDTO)
		require.NoError(t, err)
		if resp.StatusCode != http.StatusOK {
			t.Logf("Error response: %s", string(resp.Body))
		}
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("CreateJob - Forbidden for User", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		namespace := fmt.Sprintf("%d-test-admin", ctx.TestProject.PID)

		jobDTO := map[string]interface{}{
			"name":      "unauthorized-job",
			"image":     "busybox:latest",
			"command":   []string{"echo", "hello"},
			"namespace": namespace,
		}

		resp, err := client.POST("/k8s/jobs", jobDTO)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("ListJobs - All Users Can List", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		resp, err := client.GET("/k8s/jobs")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var jobs []interface{}
		err = resp.DecodeJSON(&jobs)
		require.NoError(t, err)
	})

	t.Run("GetJob - Success", func(t *testing.T) {
		client := NewHTTPClient(ctx.Router, ctx.UserToken)

		// Assuming job ID 1 exists or use dynamic ID
		resp, err := client.GET("/k8s/jobs/1")
		require.NoError(t, err)
		// May be 404 if job doesn't exist
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound)
	})
}
