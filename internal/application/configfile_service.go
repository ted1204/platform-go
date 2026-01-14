package application

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/domain/configfile"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/pkg/utils"
)

var (
	ErrConfigFileNotFound   = errors.New("config file not found")
	ErrYAMLParsingFailed    = errors.New("YAML parsing failed")
	ErrNoValidYAMLDocument  = errors.New("no valid YAML documents found")
	ErrUploadYAMLFailed     = errors.New("failed to upload YAML file")
	ErrInvalidResourceLimit = errors.New("invalid resource limit specified in YAML")
	ErrInvalidVolumeMounts  = errors.New("invalid volume/volumeMount definition in YAML")
)

type ConfigFileService struct {
	Repos        *repository.Repos
	imageService *ImageService
}

func NewConfigFileService(repos *repository.Repos) *ConfigFileService {
	return &ConfigFileService{
		Repos:        repos,
		imageService: NewImageService(repos.Image),
	}
}

func (s *ConfigFileService) ListConfigFiles() ([]configfile.ConfigFile, error) {
	return s.Repos.ConfigFile.ListConfigFiles()
}

func (s *ConfigFileService) GetConfigFile(id uint) (*configfile.ConfigFile, error) {
	return s.Repos.ConfigFile.GetConfigFileByID(id)
}

func (s *ConfigFileService) ListConfigFilesByProjectID(projectID uint) ([]configfile.ConfigFile, error) {
	return s.Repos.ConfigFile.GetConfigFilesByProjectID(projectID)
}

func (s *ConfigFileService) CreateConfigFile(c *gin.Context, cf configfile.CreateConfigFileInput) (*configfile.ConfigFile, error) {
	// Performance: Parse and validate BEFORE opening a DB transaction
	resourcesToCreate, err := s.parseAndValidateResources(cf.RawYaml)
	if err != nil {
		return nil, err
	}

	tx := s.Repos.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	createdCF := &configfile.ConfigFile{
		Filename:  cf.Filename,
		Content:   cf.RawYaml,
		ProjectID: cf.ProjectID,
	}

	if err := s.Repos.ConfigFile.WithTx(tx).CreateConfigFile(createdCF); err != nil {
		tx.Rollback()
		return nil, err
	}

	for _, res := range resourcesToCreate {
		res.CFID = createdCF.CFID
		if err := s.Repos.Resource.WithTx(tx).CreateResource(res); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to create resource %s/%s: %w", res.Type, res.Name, err)
		}
	}

	if res := tx.Commit(); res.Error != nil {
		return nil, fmt.Errorf("transaction commit failed: %w", res.Error)
	}

	logFn := utils.LogAuditWithConsole
	go func(fn func(*gin.Context, string, string, string, interface{}, interface{}, string, repository.AuditRepo)) {
		fn(c, "create", "config_file", fmt.Sprintf("cf_id=%d", createdCF.CFID), nil, *createdCF, "", s.Repos.Audit)
	}(logFn)

	return createdCF, nil
}

func (s *ConfigFileService) UpdateConfigFile(c *gin.Context, id uint, input configfile.ConfigFileUpdateDTO) (*configfile.ConfigFile, error) {
	existing, err := s.Repos.ConfigFile.GetConfigFileByID(id)
	if err != nil {
		return nil, ErrConfigFileNotFound
	}

	oldCF := *existing

	if input.Filename != nil {
		existing.Filename = *input.Filename
	}

	if input.RawYaml != nil {
		// Prepare new resources first
		newResources, err := s.parseAndValidateResources(*input.RawYaml)
		if err != nil {
			return nil, err
		}

		// Use helper to handle the diff logic (delete old, create/update new)
		// We pass the parsed resources to avoid re-parsing inside the helper
		if err = s.syncConfigFileResources(c, existing, *input.RawYaml, newResources); err != nil {
			return nil, err
		}
		existing.Content = *input.RawYaml
	}

	err = s.Repos.ConfigFile.UpdateConfigFile(existing)
	if err != nil {
		return nil, err
	}

	utils.LogAuditWithConsole(c, "update", "config_file", fmt.Sprintf("cf_id=%d", existing.CFID), oldCF, *existing, "", s.Repos.Audit)

	return existing, nil
}

func (s *ConfigFileService) DeleteConfigFile(c *gin.Context, id uint) error {
	cf, err := s.Repos.ConfigFile.GetConfigFileByID(id)
	if err != nil {
		return ErrConfigFileNotFound
	}

	// 1. Clean up K8s resources
	if err := s.DeleteConfigFileInstance(id); err != nil {
		// Log warning but proceed to delete DB records if possible, or return error depending on policy
		fmt.Printf("[Warning] Failed to cleanup K8s resources for CF %d: %v\n", id, err)
	}

	// 2. Clean up DB resources
	resources, err := s.Repos.Resource.ListResourcesByConfigFileID(id)
	if err != nil {
		return err
	}

	for _, res := range resources {
		if err := s.Repos.Resource.DeleteResource(res.RID); err != nil {
			return err
		}
	}

	// 3. Delete ConfigFile
	if err := s.Repos.ConfigFile.DeleteConfigFile(id); err != nil {
		return err
	}

	utils.LogAuditWithConsole(c, "delete", "config_file", fmt.Sprintf("cf_id=%d", cf.CFID), *cf, nil, "", s.Repos.Audit)
	return nil
}

// ValidateAndInjectGPUConfig is a thin compatibility wrapper used by unit tests.
// It unmarshals a JSON object, runs GPU validation/injection on any PodSpecs,
// and returns the resulting JSON bytes.
func (s *ConfigFileService) ValidateAndInjectGPUConfig(jsonBytes []byte, proj project.Project) ([]byte, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &obj); err != nil {
		return nil, err
	}

	podSpecs := findPodSpecs(obj)
	if len(podSpecs) == 0 {
		return jsonBytes, nil
	}

	for _, spec := range podSpecs {
		if err := s.patchGPU(spec, proj); err != nil {
			return nil, err
		}
	}

	out, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return out, nil
}
