package application_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/linskybing/platform-go/internal/application"
	"github.com/linskybing/platform-go/internal/domain/configfile"
	"github.com/linskybing/platform-go/internal/domain/group"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/domain/resource"
	"github.com/linskybing/platform-go/internal/domain/view"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/internal/repository/mock"
	"github.com/linskybing/platform-go/pkg/types"
	"github.com/linskybing/platform-go/pkg/utils"
	"gorm.io/datatypes"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func setupMocks(t *testing.T) (*application.ConfigFileService, *mock.MockConfigFileRepo,
	*mock.MockResourceRepo, *mock.MockAuditRepo,
	*mock.MockViewRepo, *mock.MockProjectRepo, *mock.MockUserGroupRepo, *gin.Context) {

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCF := mock.NewMockConfigFileRepo(ctrl)
	mockRes := mock.NewMockResourceRepo(ctrl)
	mockAudit := mock.NewMockAuditRepo(ctrl)
	mockView := mock.NewMockViewRepo(ctrl)

	mockProject := mock.NewMockProjectRepo(ctrl)
	mockUserGroup := mock.NewMockUserGroupRepo(ctrl)

	repos := &repository.Repos{
		ConfigFile: mockCF,
		Resource:   mockRes,
		Audit:      mockAudit,
		View:       mockView,
		Project:    mockProject,
		UserGroup:  mockUserGroup,
	}
	svc := application.NewConfigFileService(repos)

	// gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("POST", "/", nil)
	c.Request = req
	c.Set("claims", &types.Claims{Username: "testuser", UserID: 1})

	// mock utils
	utils.SplitYAMLDocuments = func(yamlStr string) []string { return []string{"doc1"} }
	utils.YAMLToJSON = func(doc string) (string, error) { return `{"kind":"Pod","metadata":{"name":"testpod"}}`, nil }
	utils.ValidateK8sJSON = func(jsonStr string) (*schema.GroupVersionKind, string, error) {
		return &schema.GroupVersionKind{Kind: "Pod"}, "testpod", nil
	}
	utils.LogAuditWithConsole = func(c *gin.Context, action, resourceType, resourceID string, oldData, newData interface{}, msg string, repos repository.AuditRepo) {
	}
	utils.CreateByJson = func(yaml []byte, ns string) error { return nil }
	utils.DeleteByJson = func(yaml []byte, ns string) error { return nil }
	utils.FormatNamespaceName = func(projectID uint, username string) string { return "ns-test" }

	return svc, mockCF, mockRes, mockAudit, mockView, mockProject, mockUserGroup, c
}

func TestCreateConfigFile_Success(t *testing.T) {
	svc, mockCF, mockRes, mockAudit, _, _, _, c := setupMocks(t)

	mockCF.EXPECT().CreateConfigFile(gomock.Any()).Return(nil)
	mockRes.EXPECT().CreateResource(gomock.Any()).Return(nil).AnyTimes()
	mockAudit.EXPECT().CreateAuditLog(gomock.Any()).Return(nil).AnyTimes()

	input := configfile.CreateConfigFileInput{
		Filename:  "test.yaml",
		RawYaml:   "kind: Pod\nmetadata:\n  name: testpod",
		ProjectID: 1,
	}

	cf, err := svc.CreateConfigFile(c, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cf.Filename != "test.yaml" {
		t.Fatalf("expected filename test.yaml, got %s", cf.Filename)
	}
}

func TestCreateConfigFile_NoYAMLDocuments(t *testing.T) {
	svc, _, _, _, _, _, _, c := setupMocks(t)

	utils.SplitYAMLDocuments = func(yamlStr string) []string { return []string{} }

	input := configfile.CreateConfigFileInput{
		Filename:  "test.yaml",
		RawYaml:   "",
		ProjectID: 1,
	}

	_, err := svc.CreateConfigFile(c, input)
	if !errors.Is(err, application.ErrNoValidYAMLDocument) {
		t.Fatalf("expected ErrNoValidYAMLDocument, got %v", err)
	}
}

func TestUpdateConfigFile_Success(t *testing.T) {
	svc, mockCF, mockRes, mockAudit, mockView, _, _, c := setupMocks(t)

	// Mock original ConfigFile
	existingCF := &configfile.ConfigFile{
		CFID:      1,
		ProjectID: 1,
		Filename:  "old.yaml",
	}
	mockCF.EXPECT().GetConfigFileByID(uint(1)).Return(existingCF, nil)
	mockCF.EXPECT().UpdateConfigFile(gomock.Any()).Return(nil)

	// Mock Resource
	mockRes.EXPECT().ListResourcesByConfigFileID(uint(1)).Return([]resource.Resource{}, nil)
	mockRes.EXPECT().CreateResource(gomock.Any()).Return(nil).AnyTimes()
	mockRes.EXPECT().UpdateResource(gomock.Any()).Return(nil).AnyTimes()
	mockRes.EXPECT().DeleteResource(gomock.Any()).Return(nil).AnyTimes()

	// Mock ViewRepo
	mockView.EXPECT().ListUsersByProjectID(uint(1)).Return([]view.ProjectUserView{
		{Username: "user1"},
	}, nil)

	// Mock Audit
	mockAudit.EXPECT().CreateAuditLog(gomock.Any()).Return(nil).AnyTimes()

	// Mock utils
	utils.SplitYAMLDocuments = func(yamlStr string) []string {
		return []string{"doc1"}
	}
	utils.YAMLToJSON = func(doc string) (string, error) {
		return `{"kind":"Pod","metadata":{"name":"testpod"}}`, nil
	}
	utils.ValidateK8sJSON = func(jsonStr string) (*schema.GroupVersionKind, string, error) {
		return &schema.GroupVersionKind{Kind: "Pod"}, "testpod", nil
	}
	utils.LogAuditWithConsole = func(c *gin.Context, action, resourceType, resourceID string, oldData, newData interface{}, msg string, repos repository.AuditRepo) {
	}
	utils.DeleteByJson = func(yaml []byte, ns string) error { return nil }

	filename := "new.yaml"
	rawYaml := "kind: Pod\nmetadata:\n  name: testpod"
	input := configfile.ConfigFileUpdateDTO{
		Filename: &filename,
		RawYaml:  &rawYaml,
	}

	cf, err := svc.UpdateConfigFile(c, 1, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cf.Filename != "new.yaml" {
		t.Fatalf("expected filename new.yaml, got %s", cf.Filename)
	}
}

func TestDeleteConfigFile_Success(t *testing.T) {
	svc, mockCF, mockRes, mockAudit, mockView, _, _, c := setupMocks(t)

	mockCF.EXPECT().GetConfigFileByID(uint(1)).Return(&configfile.ConfigFile{
		CFID: 1, ProjectID: 1, Filename: "test.yaml",
	}, nil)

	mockRes.EXPECT().ListResourcesByConfigFileID(uint(1)).Return([]resource.Resource{
		{RID: 10, Name: "res1"},
	}, nil)

	mockView.EXPECT().ListUsersByProjectID(uint(1)).Return([]view.ProjectUserView{
		{Username: "user1"},
	}, nil)

	mockRes.EXPECT().DeleteResource(uint(10)).Return(nil)
	mockCF.EXPECT().DeleteConfigFile(uint(1)).Return(nil)
	mockAudit.EXPECT().CreateAuditLog(gomock.Any()).Return(nil).AnyTimes()

	utils.DeleteByJson = func(yaml []byte, ns string) error {
		return nil
	}

	err := svc.DeleteConfigFile(c, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateInstance_Success(t *testing.T) {
	svc, mockCF, mockRes, _, _, mockProject, mockUserGroup, c := setupMocks(t)

	mockRes.EXPECT().ListResourcesByConfigFileID(uint(1)).Return([]resource.Resource{{RID: 1, ParsedYAML: datatypes.JSON([]byte("{}"))}}, nil)
	mockCF.EXPECT().GetConfigFileByID(uint(1)).Return(&configfile.ConfigFile{CFID: 1, ProjectID: 1}, nil)
	mockProject.EXPECT().GetProjectByID(uint(1)).Return(project.Project{PID: 1, GID: 10}, nil)
	mockUserGroup.EXPECT().GetUserGroup(uint(1), uint(10)).Return(group.UserGroup{UID: 1, GID: 10, Role: "admin"}, nil)

	err := svc.CreateInstance(c, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteInstance_Success(t *testing.T) {
	svc, mockCF, mockRes, _, _, _, _, c := setupMocks(t)

	mockRes.EXPECT().ListResourcesByConfigFileID(uint(1)).Return([]resource.Resource{{RID: 1, ParsedYAML: datatypes.JSON([]byte("{}"))}}, nil)
	mockCF.EXPECT().GetConfigFileByID(uint(1)).Return(&configfile.ConfigFile{CFID: 1, ProjectID: 1}, nil)

	err := svc.DeleteInstance(c, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteConfigFileInstance_Success(t *testing.T) {
	svc, mockCF, mockRes, _, mockView, _, _, _ := setupMocks(t)

	// Mock ConfigFile
	mockCF.EXPECT().GetConfigFileByID(uint(1)).Return(&configfile.ConfigFile{
		CFID:      1,
		ProjectID: 1,
		Filename:  "test.yaml",
	}, nil)

	// Mock Resource
	mockRes.EXPECT().ListResourcesByConfigFileID(uint(1)).Return([]resource.Resource{
		{RID: 1, Name: "res1"},
	}, nil)

	// Mock ViewRepo
	mockView.EXPECT().ListUsersByProjectID(uint(1)).Return([]view.ProjectUserView{
		{Username: "user1"},
	}, nil)

	// Mock utils
	utils.FormatNamespaceName = func(projectID uint, username string) string {
		return fmt.Sprintf("ns-%d-%s", projectID, username)
	}
	utils.DeleteByJson = func(yaml []byte, ns string) error { return nil }

	err := svc.DeleteConfigFileInstance(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValidateAndInjectGPUConfig tests MPS configuration validation and injection
func TestValidateAndInjectGPUConfig(t *testing.T) {
	svc, _, _, _, _, _, _, _ := setupMocks(t)

	t.Run("GPUConfig_WithoutGPURequest", func(t *testing.T) {
		// Pod spec without GPU request should pass through unchanged
		podJSON := `{
			"kind": "Pod",
			"metadata": {"name": "test-pod"},
			"spec": {
				"containers": [{
					"name": "test-container",
					"image": "test:latest",
					"resources": {
						"requests": {
							"cpu": "1"
						}
					}
				}]
			}
		}`

		proj := project.Project{
			PID:         1,
			ProjectName: "test-proj",
			GPUQuota:    10,
			MPSMemory:   1024,
		}

		result, err := svc.ValidateAndInjectGPUConfig([]byte(podJSON), proj)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		var obj map[string]interface{}
		if err := json.Unmarshal(result, &obj); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}

		// Verify no GPU-related fields were added
		spec := obj["spec"].(map[string]interface{})
		containers := spec["containers"].([]interface{})
		container := containers[0].(map[string]interface{})
		env, ok := container["env"]
		if ok && env != nil {
			t.Fatalf("expected no env injection for non-GPU pod, but got: %v", env)
		}
	})

	t.Run("GPUConfig_WithGPURequest_ValidConfig", func(t *testing.T) {
		// Pod spec with GPU request and valid project MPS config
		podJSON := `{
			"kind": "Pod",
			"metadata": {"name": "gpu-pod"},
			"spec": {
				"containers": [{
					"name": "gpu-container",
					"image": "cuda:latest",
					"resources": {
						"requests": {
							"nvidia.com/gpu": "1"
						}
					}
				}]
			}
		}`

		proj := project.Project{
			PID:         1,
			ProjectName: "test-proj",
			GPUQuota:    10,
			MPSMemory:   2048,
		}

		result, err := svc.ValidateAndInjectGPUConfig([]byte(podJSON), proj)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		var obj map[string]interface{}
		if err := json.Unmarshal(result, &obj); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}

		// Verify GPU configuration was injected
		spec := obj["spec"].(map[string]interface{})
		containers := spec["containers"].([]interface{})
		container := containers[0].(map[string]interface{})
		resources := container["resources"].(map[string]interface{})

		// Check resource limits were set
		limits := resources["limits"].(map[string]interface{})
		if val, ok := limits["nvidia.com/gpu"]; !ok {
			t.Fatalf("expected nvidia.com/gpu limit to be injected")
		} else if val != "10" {
			t.Fatalf("expected nvidia.com/gpu limit 10, got %v", val)
		}

		// Check environment variables were set
		env := container["env"].([]interface{})
		if len(env) < 2 {
			t.Fatalf("expected at least 2 env vars for GPU, got %d", len(env))
		}
		// Verify GPU_QUOTA and memory env
		foundQuota := false
		foundMemory := false
		for _, e := range env {
			if item, ok := e.(map[string]interface{}); ok {
				switch item["name"] {
				case "GPU_QUOTA":
					if item["value"] == "10" {
						foundQuota = true
					}
				case "CUDA_MPS_PINNED_DEVICE_MEM_LIMIT":
					if item["value"] == "2147483648" {
						foundMemory = true
					}
				}
			}
		}
		if !foundQuota {
			t.Fatalf("GPU_QUOTA env not injected or incorrect")
		}
		if !foundMemory {
			t.Fatalf("CUDA_MPS_PINNED_DEVICE_MEM_LIMIT env not injected or incorrect")
		}
	})

	t.Run("GPUConfig_InvalidGPUQuota", func(t *testing.T) {
		podJSON := `{
			"kind": "Pod",
			"metadata": {"name": "gpu-pod"},
			"spec": {
				"containers": [{
					"name": "gpu-container",
					"image": "cuda:latest",
					"resources": {
						"requests": {
							"nvidia.com/gpu": "1"
						}
					}
				}]
			}
		}`

		proj := project.Project{
			PID:         1,
			ProjectName: "test-proj",
			GPUQuota:    0, // Invalid: must be > 0
			MPSMemory:   2048,
		}

		_, err := svc.ValidateAndInjectGPUConfig([]byte(podJSON), proj)
		if err == nil {
			t.Fatalf("expected error for invalid MPS limit, but got nil")
		}
	})

	t.Run("GPUConfig_InvalidMPSMemory", func(t *testing.T) {
		podJSON := `{
			"kind": "Pod",
			"metadata": {"name": "gpu-pod"},
			"spec": {
				"containers": [{
					"name": "gpu-container",
					"image": "cuda:latest",
					"resources": {
						"requests": {
							"nvidia.com/gpu": "1"
						}
					}
				}]
			}
		}`

		proj := project.Project{
			PID:         1,
			ProjectName: "test-proj",
			GPUQuota:    10,
			MPSMemory:   256, // Invalid: < 512MB minimum
		}

		_, err := svc.ValidateAndInjectGPUConfig([]byte(podJSON), proj)
		if err == nil {
			t.Fatalf("expected error for insufficient MPS memory, but got nil")
		}
	})

	t.Run("GPUConfig_MPSMemoryOptional", func(t *testing.T) {
		podJSON := `{
			"kind": "Pod",
			"metadata": {"name": "gpu-pod"},
			"spec": {
				"containers": [{
					"name": "gpu-container",
					"image": "cuda:latest",
					"resources": {
						"requests": {
							"nvidia.com/gpu": "1"
						}
					}
				}]
			}
		}`

		proj := project.Project{
			PID:         1,
			ProjectName: "test-proj",
			GPUQuota:    5,
			MPSMemory:   0, // Optional
		}

		result, err := svc.ValidateAndInjectGPUConfig([]byte(podJSON), proj)
		if err != nil {
			t.Fatalf("expected no error for optional MPS memory, got: %v", err)
		}

		var obj map[string]interface{}
		if err := json.Unmarshal(result, &obj); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}

		spec := obj["spec"].(map[string]interface{})
		containers := spec["containers"].([]interface{})
		container := containers[0].(map[string]interface{})
		resources := container["resources"].(map[string]interface{})
		limits := resources["limits"].(map[string]interface{})
		if limits["nvidia.com/gpu"] != "5" {
			t.Fatalf("expected GPU limit 5, got %v", limits["nvidia.com/gpu"])
		}
		env := container["env"].([]interface{})
		foundQuota := false
		foundMem := false
		for _, e := range env {
			if item, ok := e.(map[string]interface{}); ok {
				if item["name"] == "GPU_QUOTA" && item["value"] == "5" {
					foundQuota = true
				}
				if item["name"] == "CUDA_MPS_PINNED_DEVICE_MEM_LIMIT" {
					foundMem = true
				}
			}
		}
		if !foundQuota {
			t.Fatalf("GPU_QUOTA env not injected or incorrect")
		}
		if foundMem {
			t.Fatalf("did not expect CUDA_MPS_PINNED_DEVICE_MEM_LIMIT when memory is 0")
		}
	})
}

func TestConfigFileRead(t *testing.T) {
	svc, mockCF, _, _, _, _, _, _ := setupMocks(t)

	t.Run("ListConfigFiles", func(t *testing.T) {
		cfs := []configfile.ConfigFile{{CFID: 1, Filename: "f1"}}
		mockCF.EXPECT().ListConfigFiles().Return(cfs, nil)

		res, err := svc.ListConfigFiles()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 config file, got %d", len(res))
		}
	})

	t.Run("GetConfigFile", func(t *testing.T) {
		cf := &configfile.ConfigFile{CFID: 1, Filename: "f1"}
		mockCF.EXPECT().GetConfigFileByID(uint(1)).Return(cf, nil)

		res, err := svc.GetConfigFile(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Filename != "f1" {
			t.Fatalf("expected f1, got %s", res.Filename)
		}
	})

	t.Run("ListConfigFilesByProjectID", func(t *testing.T) {
		cfs := []configfile.ConfigFile{{CFID: 1, Filename: "f1"}}
		mockCF.EXPECT().GetConfigFilesByProjectID(uint(10)).Return(cfs, nil)

		res, err := svc.ListConfigFilesByProjectID(10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 config file, got %d", len(res))
		}
	})
}
