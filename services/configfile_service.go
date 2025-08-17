package services

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/models"
	"github.com/linskybing/platform-go/repositories"
	"github.com/linskybing/platform-go/types"
	"github.com/linskybing/platform-go/utils"

	"gorm.io/datatypes"
)

var (
	ErrConfigFileNotFound  = errors.New("config file not found")
	ErrYAMLParsingFailed   = errors.New("YAML parsing failed")
	ErrNoValidYAMLDocument = errors.New("no valid YAML documents found")
	ErrUploadYAMLFailed    = errors.New("failed to upload YAML file")
)

type ConfigFileService struct {
	Repos *repositories.Repos
}

func NewConfigFileService(repos *repositories.Repos) *ConfigFileService {
	return &ConfigFileService{
		Repos: repos,
	}
}

func (s *ConfigFileService) ListConfigFiles() ([]models.ConfigFile, error) {
	return s.Repos.ConfigFile.ListConfigFiles()
}

func (s *ConfigFileService) GetConfigFile(id uint) (*models.ConfigFile, error) {
	return s.Repos.ConfigFile.GetConfigFileByID(id)
}

func (s *ConfigFileService) CreateConfigFile(c *gin.Context, cf dto.CreateConfigFileInput) (*models.ConfigFile, error) {
	yamlArray := utils.SplitYAMLDocuments(cf.RawYaml)
	if len(yamlArray) == 0 {
		return nil, ErrNoValidYAMLDocument
	}

	var resources []*models.Resource
	for i, doc := range yamlArray {
		jsonContent, err := utils.YAMLToJSON(doc)
		if err != nil {
			return nil, fmt.Errorf("failed to convert YAML to JSON for document %d: %w", i+1, err)
		}

		gvk, name, err := utils.ValidateK8sJSON(jsonContent)
		if err != nil {
			return nil, fmt.Errorf("failed to validate YAML document %d: %w", i+1, err)
		}

		resources = append(resources, &models.Resource{
			Type:       gvk.Kind,
			Name:       name,
			ParsedYAML: datatypes.JSON([]byte(jsonContent)),
		})
	}

	createdCF := &models.ConfigFile{
		Filename:  cf.Filename,
		ProjectID: cf.ProjectID,
	}
	if err := s.Repos.ConfigFile.CreateConfigFile(createdCF); err != nil {
		return nil, err
	}
	utils.LogAuditWithConsole(c, "create", "config_file", fmt.Sprintf("cf_id=%d", createdCF.CFID), nil, *createdCF, "", s.Repos.Audit)

	for _, res := range resources {
		res.CFID = createdCF.CFID
		if err := s.Repos.Resource.CreateResource(res); err != nil {
			return nil, fmt.Errorf("failed to create resource %s/%s: %w", res.Type, res.Name, err)
		}
		utils.LogAuditWithConsole(c, "create", "resource", fmt.Sprintf("r_id=%d", res.RID), nil, *res, "", s.Repos.Audit)
	}

	return createdCF, nil
}

func (s *ConfigFileService) updateYamlContent(c *gin.Context, cf *models.ConfigFile, rawYaml string) error {
	yamlArray := utils.SplitYAMLDocuments(rawYaml)
	if len(yamlArray) == 0 {
		return ErrNoValidYAMLDocument
	}

	resources, err := s.Repos.Resource.ListResourcesByConfigFileID(cf.CFID)
	if err != nil {
		return fmt.Errorf("failed to list resources for config file %d: %w", cf.CFID, err)
	}
	resourceMap := make(map[string]models.Resource)
	usedKeys := make(map[string]bool)
	for _, r := range resources {
		resourceMap[r.Name] = r
		usedKeys[r.Name] = false
	}
	for i, doc := range yamlArray {
		jsonContent, err := utils.YAMLToJSON(doc)
		if err != nil {
			return fmt.Errorf("failed to convert YAML to JSON for document %d: %w", i+1, err)
		}

		gvk, name, err := utils.ValidateK8sJSON(jsonContent)
		if err != nil {
			return fmt.Errorf("failed to validate YAML document %d: %w", i+1, err)
		}
		val, ok := resourceMap[name]
		var resource *models.Resource
		if !ok {
			resource = &models.Resource{
				CFID:       cf.CFID,
				Type:       gvk.Kind,
				Name:       name,
				ParsedYAML: datatypes.JSON([]byte(jsonContent)),
			}
			fmt.Printf("update resource for ccc document %d: %s", i+1, name)
			if err := s.Repos.Resource.CreateResource(resource); err != nil {
				return fmt.Errorf("failed to create resource for document %d: %w", i+1, err)
			}
			utils.LogAuditWithConsole(c, "create", "resource", fmt.Sprintf("r_id=%d", resource.RID), nil, *resource, "", s.Repos.Audit)
		} else {
			usedKeys[name] = true
			oldTarget := val
			val.Name = name
			val.ParsedYAML = datatypes.JSON([]byte(jsonContent))
			fmt.Printf("update resource for document %d: %s", i+1, name)
			if err := s.Repos.Resource.UpdateResource(&val); err != nil {
				return fmt.Errorf("failed to update resource for document %d: %w", i+1, err)
			}
			utils.LogAuditWithConsole(c, "update", "resource", fmt.Sprintf("r_id=%d", val.RID), oldTarget, val, "", s.Repos.Audit)
		}
	}

	for key, val := range usedKeys {
		if !val {
			if err := s.Repos.Resource.DeleteResource(resourceMap[key].RID); err != nil {
				return fmt.Errorf("failed to delete unused resource %s: %w", key, err)
			}
			utils.LogAuditWithConsole(c, "delete", "resource", fmt.Sprintf("r_id=%d", resourceMap[key].RID), resourceMap[key], nil, "", s.Repos.Audit)
		}
	}
	return nil
}

func (s *ConfigFileService) UpdateConfigFile(c *gin.Context, id uint, input dto.ConfigFileUpdateDTO) (*models.ConfigFile, error) {
	existing, err := s.Repos.ConfigFile.GetConfigFileByID(id)
	if err != nil {
		return nil, ErrConfigFileNotFound
	}

	oldCF := *existing

	if input.Filename != nil {
		existing.Filename = *input.Filename
	}

	if input.RawYaml != nil {
		if err := s.DeleteConfigFileInstance(id); err != nil {
			return nil, err
		}
		if err := s.updateYamlContent(c, existing, *input.RawYaml); err != nil {
			return nil, err
		}
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

	resources, err := s.Repos.Resource.ListResourcesByConfigFileID(id)
	if err != nil {
		return err
	}

	users, err := s.Repos.View.ListUsersByProjectID(cf.ProjectID)
	if err != nil {
		return err
	}

	for _, user := range users {
		ns := utils.FormatNamespaceName(cf.ProjectID, user.Username)
		for _, val := range resources {
			if err := utils.DeleteByJson(val.ParsedYAML, ns); err != nil {
				return err
			}
		}
	}

	for _, res := range resources {
		err := s.Repos.Resource.DeleteResource(res.RID)
		if err != nil {
			return err
		}
	}

	err = s.Repos.ConfigFile.DeleteConfigFile(id)
	if err != nil {
		return err
	}

	utils.LogAuditWithConsole(c, "delete", "config_file", fmt.Sprintf("cf_id=%d", cf.CFID), *cf, nil, "", s.Repos.Audit)
	return nil
}

func (s *ConfigFileService) ListConfigFilesByProjectID(projectID uint) ([]models.ConfigFile, error) {
	return s.Repos.ConfigFile.GetConfigFilesByProjectID(projectID)
}

func (s *ConfigFileService) CreateInstance(c *gin.Context, id uint) error {
	data, err := s.Repos.Resource.ListResourcesByConfigFileID(id)
	if err != nil {
		return err
	}
	configfile, err := s.Repos.ConfigFile.GetConfigFileByID(id)
	if err != nil {
		return err
	}
	claims, _ := c.MustGet("claims").(*types.Claims)
	ns := utils.FormatNamespaceName(configfile.ProjectID, claims.Username)
	for _, val := range data {
		if err := utils.CreateByJson(val.ParsedYAML, ns); err != nil {
			return err
		}
	}
	return nil
}

func (s *ConfigFileService) DeleteInstance(c *gin.Context, id uint) error {
	data, err := s.Repos.Resource.ListResourcesByConfigFileID(id)
	if err != nil {
		return err
	}
	configfile, err := s.Repos.ConfigFile.GetConfigFileByID(id)
	if err != nil {
		return err
	}
	claims, _ := c.MustGet("claims").(*types.Claims)
	ns := utils.FormatNamespaceName(configfile.ProjectID, claims.Username)
	for _, val := range data {
		if err := utils.DeleteByJson(val.ParsedYAML, ns); err != nil {
			return err
		}
	}
	return nil
}

func (s *ConfigFileService) DeleteConfigFileInstance(id uint) error {
	configfile, err := s.Repos.ConfigFile.GetConfigFileByID(id)
	if err != nil {
		return err
	}

	resources, err := s.Repos.Resource.ListResourcesByConfigFileID(id)
	if err != nil {
		return err
	}

	users, err := s.Repos.View.ListUsersByProjectID(configfile.ProjectID)
	if err != nil {
		return err
	}

	for _, user := range users {
		ns := utils.FormatNamespaceName(configfile.ProjectID, user.Username)
		for _, res := range resources {
			if err := utils.DeleteByJson(res.ParsedYAML, ns); err != nil {
				return err
			}
		}
	}

	return nil
}
