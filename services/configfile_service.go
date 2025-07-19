package services

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/dto"
	"github.com/linskybing/platform-go/models"
	"github.com/linskybing/platform-go/repositories"
	"github.com/linskybing/platform-go/utils"
	"gorm.io/datatypes"
)

var (
	ErrConfigFileNotFound  = errors.New("config file not found")
	ErrYAMLParsingFailed   = errors.New("YAML parsing failed")
	ErrNoValidYAMLDocument = errors.New("no valid YAML documents found")
	ErrUploadYAMLFailed    = errors.New("failed to upload YAML file")
)

func ListConfigFiles() ([]models.ConfigFile, error) {
	return repositories.ListConfigFiles()
}

func GetConfigFile(id uint) (*models.ConfigFile, error) {
	return repositories.GetConfigFileByID(id)
}

func CreateConfigFile(c *gin.Context, cf dto.CreateConfigFileInput) (*models.ConfigFile, error) {

	minioPath, err := utils.CreateYamlFile(c, "config", cf.RawYaml)
	if err != nil {
		return nil, ErrUploadYAMLFailed
	}

	createdCF := &models.ConfigFile{
		Filename:  cf.Filename,
		MinIOPath: minioPath,
		ProjectID: cf.ProjectID,
	}

	if err := repositories.CreateConfigFile(createdCF); err != nil {
		return nil, err
	}

	userID, _ := utils.GetUserIDFromContext(c)
	_ = utils.LogAudit(c, userID, "create", "config_file", createdCF.CFID, nil, createdCF, "")

	yamlArray := utils.SplitYAMLDocuments(cf.RawYaml)
	if len(yamlArray) == 0 {
		return nil, ErrNoValidYAMLDocument
	}

	for i, doc := range yamlArray {
		jsonContent, err := utils.YAMLToJSON(doc)
		if err != nil {
			return nil, fmt.Errorf("failed to convert YAML to JSON for document %d: %w", i+1, err)
		}

		gvk, name, err := utils.ValidateK8sJSON(jsonContent)
		if err != nil {
			return nil, fmt.Errorf("failed to validate YAML document %d: %w", i+1, err)
		}

		resource := &models.Resource{
			CFID:       createdCF.CFID,
			Type:       gvk.Kind,
			Name:       name,
			ParsedYAML: datatypes.JSON([]byte(jsonContent)),
		}

		if _, err := CreateResource(c, resource); err != nil {
			return nil, fmt.Errorf("failed to create resource for document %d: %w", i+1, err)
		}

		_ = utils.LogAudit(c, userID, "create", "resource", resource.RID, nil, resource, "")
	}
	return createdCF, nil
}

func updateYamlContent(c *gin.Context, cf *models.ConfigFile, rawYaml string) error {
	minioPath, err := utils.CreateYamlFile(c, "config", rawYaml)
	if err != nil {
		return ErrUploadYAMLFailed
	}

	cf.MinIOPath = minioPath

	yamlArray := utils.SplitYAMLDocuments(rawYaml)
	if len(yamlArray) == 0 {
		return ErrNoValidYAMLDocument
	}

	userID, _ := utils.GetUserIDFromContext(c)
	for i, doc := range yamlArray {
		jsonContent, err := utils.YAMLToJSON(doc)
		if err != nil {
			return fmt.Errorf("failed to convert YAML to JSON for document %d: %w", i+1, err)
		}

		gvk, name, err := utils.ValidateK8sJSON(jsonContent)
		if err != nil {
			return fmt.Errorf("failed to validate YAML document %d: %w", i+1, err)
		}
		target, err := repositories.GetResourceByConfigFileIDAndName(cf.CFID, name)
		if err != nil {
			return fmt.Errorf("failed to get resource for document %d: %w", i, err)
		}

		var resource *models.Resource

		if target == nil {
			resource = &models.Resource{
				CFID:       cf.CFID,
				Type:       gvk.Kind,
				Name:       name,
				ParsedYAML: datatypes.JSON([]byte(jsonContent)),
			}
			if err := repositories.CreateResource(resource); err != nil {
				return fmt.Errorf("failed to create resource for document %d: %w", i+1, err)
			}
			_ = utils.LogAudit(c, userID, "create", "resource", resource.RID, nil, resource, "")
		} else {
			oldTarget := *target
			target.Name = name
			target.ParsedYAML = datatypes.JSON([]byte(jsonContent))
			if err := repositories.UpdateResource(target); err != nil {
				return fmt.Errorf("failed to update resource for document %d: %w", i+1, err)
			}
			_ = utils.LogAudit(c, userID, "update", "resource", target.RID, oldTarget, target, "")
		}
	}
	return nil
}
func UpdateConfigFile(c *gin.Context, id uint, input dto.ConfigFileUpdateDTO) (*models.ConfigFile, error) {
	existing, err := repositories.GetConfigFileByID(id)
	if err != nil {
		return nil, ErrConfigFileNotFound
	}

	oldCF := *existing

	if input.Filename != nil {
		existing.Filename = *input.Filename
	}

	if input.RawYaml != nil {
		if err := updateYamlContent(c, existing, *input.RawYaml); err != nil {
			return nil, err
		}
	}

	err = repositories.UpdateConfigFile(existing)
	if err != nil {
		return nil, err
	}

	userID, _ := utils.GetUserIDFromContext(c)
	_ = utils.LogAudit(c, userID, "update", "config_file", existing.CFID, oldCF, *existing, "")

	return existing, nil
}

func DeleteConfigFile(c *gin.Context, id uint) error {
	cf, err := repositories.GetConfigFileByID(id)
	if err != nil {
		return ErrConfigFileNotFound
	}

	resources, err := ListResourcesByConfigFileID(id)
	if err != nil {
		return err
	}

	for _, res := range resources {
		err := DeleteResource(c, res.RID)
		if err != nil {
			return err
		}
	}

	err = repositories.DeleteConfigFile(id)
	if err != nil {
		return err
	}

	userID, _ := utils.GetUserIDFromContext(c)
	_ = utils.LogAudit(c, userID, "delete", "config_file", cf.CFID, *cf, nil, "")

	return nil
}

func ListConfigFilesByProjectID(projectID uint) ([]models.ConfigFile, error) {
	return repositories.GetConfigFilesByProjectID(projectID)
}
