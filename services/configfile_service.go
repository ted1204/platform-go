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

	utils.LogAuditWithConsole(c, "create", "config_file", fmt.Sprintf("cf_id=%d", createdCF.CFID), nil, *createdCF, "")
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
		utils.LogAuditWithConsole(c, "create", "resource", fmt.Sprintf("r_id=%d", resource.RID), nil, *resource, "")
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

	resources, err := repositories.ListResourcesByConfigFileID(cf.CFID)
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
			if err := repositories.CreateResource(resource); err != nil {
				return fmt.Errorf("failed to create resource for document %d: %w", i+1, err)
			}
			utils.LogAuditWithConsole(c, "create", "resource", fmt.Sprintf("r_id=%d", resource.RID), nil, *resource, "")
		} else {
			usedKeys[name] = true
			oldTarget := val
			val.Name = name
			val.ParsedYAML = datatypes.JSON([]byte(jsonContent))
			if err := repositories.UpdateResource(&val); err != nil {
				return fmt.Errorf("failed to update resource for document %d: %w", i+1, err)
			}
			utils.LogAuditWithConsole(c, "update", "resource", fmt.Sprintf("r_id=%d", val.RID), oldTarget, val, "")
		}
	}

	for key, val := range usedKeys {
		if !val {
			if err := repositories.DeleteResource(resourceMap[key].RID); err != nil {
				return fmt.Errorf("failed to delete unused resource %s: %w", key, err)
			}
			utils.LogAuditWithConsole(c, "delete", "resource", fmt.Sprintf("r_id=%d", resourceMap[key].RID), resourceMap[key], nil, "")
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

	utils.LogAuditWithConsole(c, "update", "config_file", fmt.Sprintf("cf_id=%d", existing.CFID), oldCF, *existing, "")

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

	utils.LogAuditWithConsole(c, "delete", "config_file", fmt.Sprintf("cf_id=%d", cf.CFID), *cf, nil, "")
	return nil
}

func ListConfigFilesByProjectID(projectID uint) ([]models.ConfigFile, error) {
	return repositories.GetConfigFilesByProjectID(projectID)
}

func CreateInstance(c *gin.Context, id uint) error {
	data, err := repositories.ListResourcesByConfigFileID(id)
	if err != nil {
		return err
	}
	configfile, err := repositories.GetConfigFileByID(id)
	if err != nil {
		return err
	}
	claims, _ := c.MustGet("claims").(*types.Claims)
	ns := fmt.Sprintf("project-%d-%v", configfile.ProjectID, claims.Username)
	utils.CreateNamespace(ns)
	for _, val := range data {
		if err := utils.CreateByJson(val.ParsedYAML, ns); err != nil {
			return err
		}
	}
	return nil
}

func UpdateConfigfile(c *gin.Context, id uint) error {
	data, err := repositories.ListResourcesByConfigFileID(id)
	if err != nil {
		return err
	}
	configfile, err := repositories.GetConfigFileByID(id)
	if err != nil {
		return err
	}
	claims, _ := c.MustGet("claims").(*types.Claims)
	ns := fmt.Sprintf("project-%d-%v", configfile.ProjectID, claims.Username)
	utils.CreateNamespace(ns)
	for _, val := range data {
		if err := utils.UpdateByJson(val.ParsedYAML, ns); err != nil {
			return err
		}
	}
	return nil
}

func DeleteInstance(c *gin.Context, id uint) error {
	data, err := repositories.ListResourcesByConfigFileID(id)
	if err != nil {
		return err
	}
	configfile, err := repositories.GetConfigFileByID(id)
	if err != nil {
		return err
	}
	claims, _ := c.MustGet("claims").(*types.Claims)
	ns := fmt.Sprintf("project-%d-%v", configfile.ProjectID, claims.Username)
	for _, val := range data {
		if err := utils.DeleteByJson(val.ParsedYAML, ns); err != nil {
			return err
		}
	}
	return nil
}

//    nsName := "my-namespace"
//     ns := &corev1.Namespace{
//         ObjectMeta: metav1.ObjectMeta{
//             Name: nsName,
//         },
//     }

//     // 創建 namespace
//     createdNS, err := clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
//     if err != nil {
//         log.Fatalf("創建 namespace 失敗: %v", err)
//     }

//     fmt.Printf("成功創建 Namespace: %s\n", createdNS.Name)
