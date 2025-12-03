package services_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/models"
	"github.com/linskybing/platform-go/src/repositories"
	"github.com/linskybing/platform-go/src/repositories/mock_repositories"
	"github.com/linskybing/platform-go/src/services"
	"github.com/linskybing/platform-go/src/types"
	"github.com/linskybing/platform-go/src/utils"
	"gorm.io/datatypes"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func setupMocks(t *testing.T) (*services.ConfigFileService, *mock_repositories.MockConfigFileRepo,
	*mock_repositories.MockResourceRepo, *mock_repositories.MockAuditRepo,
	*mock_repositories.MockViewRepo, *gin.Context) {

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockCF := mock_repositories.NewMockConfigFileRepo(ctrl)
	mockRes := mock_repositories.NewMockResourceRepo(ctrl)
	mockAudit := mock_repositories.NewMockAuditRepo(ctrl)
	mockView := mock_repositories.NewMockViewRepo(ctrl)

	repos := &repositories.Repos{
		ConfigFile: mockCF,
		Resource:   mockRes,
		Audit:      mockAudit,
		View:       mockView,
	}
	svc := services.NewConfigFileService(repos)

	// gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest("POST", "/", nil)
	c.Request = req
	c.Set("claims", &types.Claims{Username: "testuser"})

	// mock utils
	utils.SplitYAMLDocuments = func(yamlStr string) []string { return []string{"doc1"} }
	utils.YAMLToJSON = func(doc string) (string, error) { return `{"kind":"Pod","metadata":{"name":"testpod"}}`, nil }
	utils.ValidateK8sJSON = func(jsonStr string) (*schema.GroupVersionKind, string, error) {
		return &schema.GroupVersionKind{Kind: "Pod"}, "testpod", nil
	}
	utils.LogAuditWithConsole = func(c *gin.Context, action, resourceType, resourceID string, oldData, newData interface{}, msg string, repos repositories.AuditRepo) {
	}
	utils.CreateByJson = func(yaml []byte, ns string) error { return nil }
	utils.DeleteByJson = func(yaml []byte, ns string) error { return nil }
	utils.FormatNamespaceName = func(projectID uint, username string) string { return "ns-test" }

	return svc, mockCF, mockRes, mockAudit, mockView, c
}

func TestCreateConfigFile_Success(t *testing.T) {
	svc, mockCF, mockRes, mockAudit, _, c := setupMocks(t)

	mockCF.EXPECT().CreateConfigFile(gomock.Any()).Return(nil)
	mockRes.EXPECT().CreateResource(gomock.Any()).Return(nil).AnyTimes()
	mockAudit.EXPECT().CreateAuditLog(gomock.Any()).Return(nil).AnyTimes()

	input := dto.CreateConfigFileInput{
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
	svc, _, _, _, _, c := setupMocks(t)

	utils.SplitYAMLDocuments = func(yamlStr string) []string { return []string{} }

	input := dto.CreateConfigFileInput{
		Filename:  "test.yaml",
		RawYaml:   "",
		ProjectID: 1,
	}

	_, err := svc.CreateConfigFile(c, input)
	if !errors.Is(err, services.ErrNoValidYAMLDocument) {
		t.Fatalf("expected ErrNoValidYAMLDocument, got %v", err)
	}
}

func TestUpdateConfigFile_Success(t *testing.T) {
	svc, mockCF, mockRes, mockAudit, mockView, c := setupMocks(t)

	// Mock original ConfigFile
	existingCF := &models.ConfigFile{
		CFID:      1,
		ProjectID: 1,
		Filename:  "old.yaml",
	}
	mockCF.EXPECT().GetConfigFileByID(uint(1)).Return(existingCF, nil)
	mockCF.EXPECT().UpdateConfigFile(gomock.Any()).Return(nil)

	// Mock Resource
	mockRes.EXPECT().ListResourcesByConfigFileID(uint(1)).Return([]models.Resource{}, nil)
	mockRes.EXPECT().CreateResource(gomock.Any()).Return(nil).AnyTimes()
	mockRes.EXPECT().UpdateResource(gomock.Any()).Return(nil).AnyTimes()
	mockRes.EXPECT().DeleteResource(gomock.Any()).Return(nil).AnyTimes()

	// Mock ViewRepo
	mockView.EXPECT().ListUsersByProjectID(uint(1)).Return([]models.ProjectUserView{
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
	utils.LogAuditWithConsole = func(c *gin.Context, action, resourceType, resourceID string, oldData, newData interface{}, msg string, repos repositories.AuditRepo) {
	}
	utils.DeleteByJson = func(yaml []byte, ns string) error { return nil }

	filename := "new.yaml"
	rawYaml := "kind: Pod\nmetadata:\n  name: testpod"
	input := dto.ConfigFileUpdateDTO{
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
	svc, mockCF, mockRes, mockAudit, mockView, c := setupMocks(t)

	mockCF.EXPECT().GetConfigFileByID(uint(1)).Return(&models.ConfigFile{
		CFID: 1, ProjectID: 1, Filename: "test.yaml",
	}, nil)

	mockRes.EXPECT().ListResourcesByConfigFileID(uint(1)).Return([]models.Resource{
		{RID: 10, Name: "res1"},
	}, nil)

	mockView.EXPECT().ListUsersByProjectID(uint(1)).Return([]models.ProjectUserView{
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
	svc, mockCF, mockRes, _, _, c := setupMocks(t)

	mockRes.EXPECT().ListResourcesByConfigFileID(uint(1)).Return([]models.Resource{{RID: 1, ParsedYAML: datatypes.JSON([]byte("{}"))}}, nil)
	mockCF.EXPECT().GetConfigFileByID(uint(1)).Return(&models.ConfigFile{CFID: 1, ProjectID: 1}, nil)

	err := svc.CreateInstance(c, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteInstance_Success(t *testing.T) {
	svc, mockCF, mockRes, _, _, c := setupMocks(t)

	mockRes.EXPECT().ListResourcesByConfigFileID(uint(1)).Return([]models.Resource{{RID: 1, ParsedYAML: datatypes.JSON([]byte("{}"))}}, nil)
	mockCF.EXPECT().GetConfigFileByID(uint(1)).Return(&models.ConfigFile{CFID: 1, ProjectID: 1}, nil)

	err := svc.DeleteInstance(c, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteConfigFileInstance_Success(t *testing.T) {
	svc, mockCF, mockRes, _, mockView, _ := setupMocks(t)

	// Mock ConfigFile
	mockCF.EXPECT().GetConfigFileByID(uint(1)).Return(&models.ConfigFile{
		CFID:      1,
		ProjectID: 1,
		Filename:  "test.yaml",
	}, nil)

	// Mock Resource
	mockRes.EXPECT().ListResourcesByConfigFileID(uint(1)).Return([]models.Resource{
		{RID: 1, Name: "res1"},
	}, nil)

	// Mock ViewRepo
	mockView.EXPECT().ListUsersByProjectID(uint(1)).Return([]models.ProjectUserView{
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

func TestConfigFileRead(t *testing.T) {
	svc, mockCF, _, _, _, _ := setupMocks(t)

	t.Run("ListConfigFiles", func(t *testing.T) {
		cfs := []models.ConfigFile{{CFID: 1, Filename: "f1"}}
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
		cf := &models.ConfigFile{CFID: 1, Filename: "f1"}
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
		cfs := []models.ConfigFile{{CFID: 1, Filename: "f1"}}
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
