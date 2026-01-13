package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/config"
	"github.com/linskybing/platform-go/internal/domain/configfile"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/domain/resource"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/pkg/k8s"
	"github.com/linskybing/platform-go/pkg/types"
	"github.com/linskybing/platform-go/pkg/utils"
	"gorm.io/datatypes"
	k8sRes "k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"
)

var (
	ErrConfigFileNotFound   = errors.New("config file not found")
	ErrYAMLParsingFailed    = errors.New("YAML parsing failed")
	ErrNoValidYAMLDocument  = errors.New("no valid YAML documents found")
	ErrUploadYAMLFailed     = errors.New("failed to upload YAML file")
	ErrInvalidResourceLimit = errors.New("invalid resource limit specified in YAML")
)

type ConfigFileService struct {
	Repos        *repository.Repos
	imageService *ImageService
}

func normalizeResourceKind(kind string) string {
	switch strings.ToLower(kind) {
	case "pod":
		return "Pod"
	case "service":
		return "Service"
	case "deployment":
		return "Deployment"
	case "configmap":
		return "ConfigMap"
	case "ingress":
		return "Ingress"
	default:
		if kind == "" {
			return kind
		}
		return strings.ToUpper(string(kind[0])) + strings.ToLower(kind[1:])
	}
}

func NewConfigFileService(repos *repository.Repos) *ConfigFileService {
	return &ConfigFileService{
		Repos:        repos,
		imageService: NewImageService(repos.Image),
	}
}

func (s *ConfigFileService) extractAndValidateImages(jsonBytes []byte, projectID uint, userIsAdmin bool) error {
	if userIsAdmin {
		return nil
	}

	var obj map[string]interface{}
	if err := yaml.Unmarshal(jsonBytes, &obj); err != nil {
		return fmt.Errorf("failed to parse resource definition: %w", err)
	}

	images := []string{}

	collectFromList := func(list interface{}) {
		if containers, ok := list.([]interface{}); ok {
			for _, c := range containers {
				if cont, ok := c.(map[string]interface{}); ok {
					if img, ok := cont["image"].(string); ok && img != "" {
						images = append(images, img)
					}
				}
			}
		}
	}

	checkPodSpec := func(spec map[string]interface{}) {
		if spec == nil {
			return
		}
		collectFromList(spec["containers"])
		collectFromList(spec["initContainers"])
	}

	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		checkPodSpec(spec)

		if template, ok := spec["template"].(map[string]interface{}); ok {
			if tSpec, ok := template["spec"].(map[string]interface{}); ok {
				checkPodSpec(tSpec)
			}
		}
	}

	for _, img := range images {
		imageName, imageTag := parseImageNameTag(img)

		allowed, err := s.imageService.ValidateImageForProject(imageName, imageTag, &projectID)
		log.Printf("Validating image: %s:%s, Allowed: %v, Error: %v", imageName, imageTag, allowed, err)
		if err != nil {
			return fmt.Errorf("failed to validate image %s: %v", img, err)
		}
		if !allowed {
			return fmt.Errorf("Image '%s:%s' is not allowed for this project.", imageName, imageTag)
		}
	}

	return nil
}

func parseImageNameTag(img string) (name string, tag string) {
	lastColon := strings.LastIndex(img, ":")
	lastSlash := strings.LastIndex(img, "/")

	if lastColon == -1 || lastColon < lastSlash {
		return img, "latest"
	}

	return img[:lastColon], img[lastColon+1:]
}

// injectHarborPrefix modifies image references to use Harbor registry for pulled images
func (s *ConfigFileService) injectHarborPrefix(jsonBytes []byte, projectID uint) ([]byte, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &obj); err != nil {
		return jsonBytes, nil
	}

	isImagePulled := func(imageName, imageTag string) bool {
		allowed, err := s.imageService.GetAllowedImage(imageName, imageTag, projectID)
		if err != nil || allowed == nil {
			return false
		}
		return allowed.IsPulled
	}

	processContainers := func(containers []interface{}) {
		for _, c := range containers {
			if cont, ok := c.(map[string]interface{}); ok {
				if img, ok := cont["image"].(string); ok {
					if strings.HasPrefix(img, config.HarborPrivatePrefix) {
						continue
					}
					parts := strings.Split(img, ":")
					if len(parts) != 2 {
						continue
					}
					imageName := parts[0]
					imageTag := parts[1]

					if isImagePulled(imageName, imageTag) {
						cont["image"] = config.HarborPrivatePrefix + img
					}
				}
			}
		}
	}

	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		if containers, ok := spec["containers"].([]interface{}); ok {
			processContainers(containers)
		}
		if template, ok := spec["template"].(map[string]interface{}); ok {
			if tSpec, ok := template["spec"].(map[string]interface{}); ok {
				if containers, ok := tSpec["containers"].([]interface{}); ok {
					processContainers(containers)
				}
			}
		}
	}

	return json.Marshal(obj)
}

func (s *ConfigFileService) ListConfigFiles() ([]configfile.ConfigFile, error) {
	return s.Repos.ConfigFile.ListConfigFiles()
}

func (s *ConfigFileService) GetConfigFile(id uint) (*configfile.ConfigFile, error) {
	return s.Repos.ConfigFile.GetConfigFileByID(id)
}

func (s *ConfigFileService) CreateConfigFile(c *gin.Context, cf configfile.CreateConfigFileInput) (*configfile.ConfigFile, error) {
	yamlArray := utils.SplitYAMLDocuments(cf.RawYaml)
	if len(yamlArray) == 0 {
		return nil, ErrNoValidYAMLDocument
	}

	var resourcesToCreate []*resource.Resource

	for i, doc := range yamlArray {
		jsonBytes, err := yaml.YAMLToJSON([]byte(doc))
		if err != nil {
			return nil, fmt.Errorf("failed to convert YAML to JSON for document %d: %w", i+1, err)
		}

		var obj map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &obj); err != nil {
			return nil, fmt.Errorf("failed to parse JSON for validation in document %d: %w", i+1, err)
		}

		if err := validateContainerLimits(obj); err != nil {
			return nil, fmt.Errorf("validation failed in document %d: %w", i+1, err)
		}

		gvk, name, err := k8s.ValidateK8sJSON(jsonBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to validate K8s spec for document %d: %w", i+1, err)
		}

		resourcesToCreate = append(resourcesToCreate, &resource.Resource{
			Type:       resource.ResourceType(normalizeResourceKind(gvk.Kind)),
			Name:       name,
			ParsedYAML: datatypes.JSON(jsonBytes),
		})
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

	go func() {
		utils.LogAuditWithConsole(c, "create", "config_file", fmt.Sprintf("cf_id=%d", createdCF.CFID), nil, *createdCF, "", s.Repos.Audit)
	}()

	return createdCF, nil
}

func validateContainerLimits(obj map[string]interface{}) error {
	if obj == nil {
		return nil
	}

	kind, _ := obj["kind"].(string)
	var podSpec map[string]interface{}

	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		switch kind {
		case "Pod":
			podSpec = spec
		case "Deployment", "StatefulSet", "DaemonSet", "Job", "ReplicaSet", "ReplicationController":
			if template, ok := spec["template"].(map[string]interface{}); ok {
				if tSpec, ok := template["spec"].(map[string]interface{}); ok {
					podSpec = tSpec
				}
			}
		case "CronJob":
			if jobTemplate, ok := spec["jobTemplate"].(map[string]interface{}); ok {
				if jobSpec, ok := jobTemplate["spec"].(map[string]interface{}); ok {
					if template, ok := jobSpec["template"].(map[string]interface{}); ok {
						if tSpec, ok := template["spec"].(map[string]interface{}); ok {
							podSpec = tSpec
						}
					}
				}
			}
		}
	}

	if podSpec == nil {
		return nil
	}

	var containers []interface{}
	if c, ok := podSpec["containers"].([]interface{}); ok {
		containers = append(containers, c...)
	}

	for _, c := range containers {
		container, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		containerName, _ := container["name"].(string)

		resources, ok := container["resources"].(map[string]interface{})
		if !ok {
			continue
		}

		requests, _ := resources["requests"].(map[string]interface{})
		limits, _ := resources["limits"].(map[string]interface{})

		if requests == nil || limits == nil {
			continue
		}

		checkResource := func(resName string) error {
			reqStr, hasReq := getStringValue(requests, resName)
			limStr, hasLim := getStringValue(limits, resName)

			if hasReq && hasLim {
				reqQ, err1 := k8sRes.ParseQuantity(reqStr)
				limQ, err2 := k8sRes.ParseQuantity(limStr)

				if err1 == nil && err2 == nil {
					if limQ.Cmp(reqQ) < 0 {
						return fmt.Errorf("container '%s': %s limit (%s) cannot be less than request (%s)",
							containerName, resName, limStr, reqStr)
					}
				}
			}
			return nil
		}

		if err := checkResource("cpu"); err != nil {
			return err
		}
		if err := checkResource("memory"); err != nil {
			return err
		}
	}

	return nil
}

func getStringValue(m map[string]interface{}, key string) (string, bool) {
	val, ok := m[key]
	if !ok {
		return "", false
	}

	switch v := val.(type) {
	case string:
		return v, true
	case float64:
		return fmt.Sprintf("%g", v), true
	case int, int64, int32:
		return fmt.Sprintf("%d", v), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func (s *ConfigFileService) updateYamlContent(c *gin.Context, cf *configfile.ConfigFile, rawYaml string, resources []resource.Resource) error {
	yamlArray := utils.SplitYAMLDocuments(rawYaml)
	if len(yamlArray) == 0 {
		return ErrNoValidYAMLDocument
	}

	resourceMap := make(map[string]resource.Resource)
	usedKeys := make(map[string]bool)
	for _, r := range resources {
		resourceMap[r.Name] = r
		usedKeys[r.Name] = false
	}
	for i, doc := range yamlArray {
		jsonContent, err := yaml.YAMLToJSON([]byte(doc))
		if err != nil {
			return fmt.Errorf("failed to convert YAML to JSON for document %d: %w", i+1, err)
		}

		gvk, name, err := k8s.ValidateK8sJSON(jsonContent)
		if err != nil {
			return fmt.Errorf("failed to validate YAML document %d: %w", i+1, err)
		}
		val, ok := resourceMap[name]
		var res *resource.Resource
		if !ok {
			res = &resource.Resource{
				CFID:       cf.CFID,
				Type:       resource.ResourceType(normalizeResourceKind(gvk.Kind)),
				Name:       name,
				ParsedYAML: datatypes.JSON([]byte(jsonContent)),
			}
			fmt.Printf("update resource for ccc document %d: %s\n", i+1, name)
			if err := s.Repos.Resource.CreateResource(res); err != nil {
				return fmt.Errorf("failed to create resource for document %d: %w", i+1, err)
			}
			utils.LogAuditWithConsole(c, "create", "resource", fmt.Sprintf("r_id=%d", res.RID), nil, *res, "", s.Repos.Audit)
		} else {
			usedKeys[name] = true
			oldTarget := val
			val.Name = name
			val.ParsedYAML = datatypes.JSON([]byte(jsonContent))
			fmt.Printf("update resource for document %d: %s\n", i+1, name)
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
		resources, err := s.deleteConfigFileInstance(existing)
		if err != nil {
			return nil, err
		}

		if err = s.updateYamlContent(c, existing, *input.RawYaml, resources); err != nil {
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

	resources, err := s.Repos.Resource.ListResourcesByConfigFileID(id)
	if err != nil {
		return err
	}

	users, err := s.Repos.User.ListUsersByProjectID(cf.ProjectID)
	if err != nil {
		return err
	}

	// Clean up K8s resources for all users
	for _, user := range users {
		ns := k8s.FormatNamespaceName(cf.ProjectID, user.Username)
		for _, val := range resources {
			// Best effort deletion
			if err := k8s.DeleteByJson(val.ParsedYAML, ns); err != nil {
				fmt.Printf("[Warning] Failed to delete resource in ns %s: %v\n", ns, err)
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

func (s *ConfigFileService) ListConfigFilesByProjectID(projectID uint) ([]configfile.ConfigFile, error) {
	return s.Repos.ConfigFile.GetConfigFilesByProjectID(projectID)
}

// CreateInstance deploys the config file resources to the Kubernetes cluster.
// It handles:
// 1. Longhorn Volume Binding (User & Project)
// 2. Permission checks (ReadOnly enforcement)
// 3. Template replacement
// 4. Image Validation & Harbor Injection
// 5. GPU Configuration
// 6. Security Context Injection
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
	safeUsername := k8s.ToSafeK8sName(claims.Username)
	targetNs := k8s.FormatNamespaceName(configfile.ProjectID, safeUsername)
	if err := k8s.EnsureNamespaceExists(targetNs); err != nil {
		return fmt.Errorf("failed to ensure namespace %s: %w", targetNs, err)
	}
	project, err := s.Repos.Project.GetProjectByID(configfile.ProjectID)
	if err != nil {
		return err
	}

	// Prepare Source Namespaces/PVCs
	userStorageNs := fmt.Sprintf(config.UserStorageNs, safeUsername)
	userPvcName := fmt.Sprintf(config.UserStoragePVC, safeUsername)

	projectStorageNs := k8s.GenerateSafeResourceName("project", project.ProjectName, project.PID)
	projectPvcName := fmt.Sprintf("project-%d-disk", project.PID)

	// 1. Bind User Volume (Personal Data)
	targetUserPvcName := userPvcName
	if err := k8s.MountExistingVolumeToProject(userStorageNs, userPvcName, targetNs, targetUserPvcName); err != nil {
		// Log warning: User might be new and has no storage yet
		fmt.Printf("[Warning] Failed to bind user volume: %v\n", err)
	}

	// 2. Bind Project Volume (Shared Data)
	targetProjectPvcName := projectPvcName
	if err := k8s.MountExistingVolumeToProject(projectStorageNs, projectPvcName, targetNs, targetProjectPvcName); err != nil {
		fmt.Printf("[Warning] Failed to bind project volume: %v\n", err)
	}

	// 3. Determine Permission (ReadOnly Check)
	shouldEnforceRO := false
	if !claims.IsAdmin {
		ug, err := s.Repos.UserGroup.GetUserGroup(claims.UserID, project.GID)
		if err != nil {
			return err
		}
		if ug.Role != "manager" && ug.Role != "admin" {
			shouldEnforceRO = true
		}
	}

	// 4. Template Variables
	templateValues := map[string]string{
		"username":         safeUsername,
		"originalUsername": claims.Username,
		"safeUsername":     safeUsername,
		"namespace":        targetNs,
		"projectId":        fmt.Sprintf("%d", configfile.ProjectID),

		"userVolume":    targetUserPvcName,    // e.g. "user-john-doe-disk"
		"projectVolume": targetProjectPvcName, // e.g. "project-101-disk"
	}

	// 5. Prepare and Validate all resources first
	var processedResources [][]byte

	for _, val := range data {
		jsonStr := string(val.ParsedYAML)

		// A. Template replacement
		replacedJSON, err := utils.ReplacePlaceholdersInJSON(jsonStr, templateValues)
		if err != nil {
			return fmt.Errorf("failed to replace placeholders: %w", err)
		}

		currentBytes := []byte(replacedJSON)

		// B. Validation: Image Whitelist
		if err := s.extractAndValidateImages(currentBytes, configfile.ProjectID, claims.IsAdmin); err != nil {
			return err
		}

		// C. Injection: Harbor Prefix
		currentBytes, err = s.injectHarborPrefix(currentBytes, configfile.ProjectID)
		if err != nil {
			return err
		}

		// D. Enforcement: ReadOnly PVC
		if shouldEnforceRO {
			currentBytes, err = s.enforceReadOnlyPVC(currentBytes, targetProjectPvcName)
			if err != nil {
				return err
			}
		}

		// E. Injection: GPU Config
		currentBytes, err = s.ValidateAndInjectGPUConfig(currentBytes, project)
		if err != nil {
			return err
		}

		// F. Injection: General Pod Config (SecurityContext, etc.)
		currentBytes, err = s.injectGeneralPodConfig(currentBytes)
		if err != nil {
			return err
		}

		processedResources = append(processedResources, currentBytes)
	}

	log.Printf("All %d resources validated. Starting creation in namespace %s", len(processedResources), targetNs)

	for _, jsonBytes := range processedResources {
		if err := k8s.CreateByJson(datatypes.JSON(jsonBytes), targetNs); err != nil {
			return fmt.Errorf("failed to create resource: %w", err)
		}
	}
	return nil
}

// enforceReadOnlyPVC parses the JSON resource and sets "readOnly: true"
// for any volume mount that points to the specified PVC name.
func (s *ConfigFileService) enforceReadOnlyPVC(jsonBytes []byte, targetPvcName string) ([]byte, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &obj); err != nil {
		return nil, err
	}

	kind, _ := obj["kind"].(string)
	var podSpec map[string]interface{}

	switch kind {
	case "Pod":
		if spec, ok := obj["spec"].(map[string]interface{}); ok {
			podSpec = spec
		}
	case "Deployment", "StatefulSet", "DaemonSet", "Job":
		if spec, ok := obj["spec"].(map[string]interface{}); ok {
			if template, ok := spec["template"].(map[string]interface{}); ok {
				if tSpec, ok := template["spec"].(map[string]interface{}); ok {
					podSpec = tSpec
				}
			}
		}
	}

	if podSpec == nil {
		return jsonBytes, nil
	}

	// Identify volumes pointing to the restricted PVC
	targetVolumes := make(map[string]bool)

	if volumes, ok := podSpec["volumes"].([]interface{}); ok {
		for _, v := range volumes {
			if vol, ok := v.(map[string]interface{}); ok {
				volName, _ := vol["name"].(string)
				if pvcSource, ok := vol["persistentVolumeClaim"].(map[string]interface{}); ok {
					claimName, _ := pvcSource["claimName"].(string)
					// Check if this volume uses the Target PVC
					if claimName == targetPvcName {
						targetVolumes[volName] = true
					}
				}
			}
		}
	}

	if len(targetVolumes) == 0 {
		return jsonBytes, nil
	}

	// Enforce ReadOnly on matched matched volume mounts
	if containers, ok := podSpec["containers"].([]interface{}); ok {
		for _, c := range containers {
			if container, ok := c.(map[string]interface{}); ok {
				if mounts, ok := container["volumeMounts"].([]interface{}); ok {
					for _, m := range mounts {
						if mount, ok := m.(map[string]interface{}); ok {
							mountName, _ := mount["name"].(string)
							if targetVolumes[mountName] {
								mount["readOnly"] = true
							}
						}
					}
				}
			}
		}
	}

	return json.Marshal(obj)
}

func (s *ConfigFileService) ValidateAndInjectGPUConfig(jsonBytes []byte, project project.Project) ([]byte, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &obj); err != nil {
		return nil, err
	}

	kind, ok := obj["kind"].(string)
	if !ok {
		return jsonBytes, nil
	}

	var podSpec map[string]interface{}

	switch kind {
	case "Pod":
		if spec, ok := obj["spec"].(map[string]interface{}); ok {
			podSpec = spec
		}
	case "Deployment", "StatefulSet", "DaemonSet", "Job":
		if spec, ok := obj["spec"].(map[string]interface{}); ok {
			if template, ok := spec["template"].(map[string]interface{}); ok {
				if tSpec, ok := template["spec"].(map[string]interface{}); ok {
					podSpec = tSpec
				}
			}
		}
	default:
		return jsonBytes, nil
	}

	if podSpec == nil {
		return jsonBytes, nil
	}

	hasGPURequest, err := s.containerHasGPURequest(podSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to check GPU requests: %w", err)
	}

	if !hasGPURequest {
		return jsonBytes, nil
	}

	if err := s.validateProjectMPSConfig(project); err != nil {
		return nil, err
	}

	if err := s.injectMPSConfig(podSpec, project); err != nil {
		return nil, fmt.Errorf("failed to inject MPS configuration: %w", err)
	}

	return json.Marshal(obj)
}

func (s *ConfigFileService) containerHasGPURequest(podSpec map[string]interface{}) (bool, error) {
	containers, ok := podSpec["containers"].([]interface{})
	if !ok {
		return false, nil
	}

	for _, c := range containers {
		if container, ok := c.(map[string]interface{}); ok {
			if resources, ok := container["resources"].(map[string]interface{}); ok {
				if requests, ok := resources["requests"].(map[string]interface{}); ok {
					if _, hasGPU := requests["nvidia.com/gpu"]; hasGPU {
						return true, nil
					}
				}
			}
		}
	}

	return false, nil
}

func (s *ConfigFileService) validateProjectMPSConfig(project project.Project) error {
	if project.GPUQuota <= 0 {
		return fmt.Errorf("project GPU configuration invalid: GPUQuota=%d. Must be greater than 0",
			project.GPUQuota)
	}

	if project.MPSMemory > 0 && project.MPSMemory < 512 {
		return fmt.Errorf("MPS memory limit too low: %dMB. Must be at least 512MB or 0 (disabled)", project.MPSMemory)
	}

	return nil
}

func (s *ConfigFileService) injectMPSConfig(podSpec map[string]interface{}, project project.Project) error {
	containers, ok := podSpec["containers"].([]interface{})
	if !ok {
		return nil
	}

	for _, c := range containers {
		container, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		resources, ok := container["resources"].(map[string]interface{})
		if !ok {
			continue
		}

		requests, ok := resources["requests"].(map[string]interface{})
		if !ok {
			continue
		}

		if _, hasGPU := requests["nvidia.com/gpu"]; !hasGPU {
			continue
		}

		limits, ok := resources["limits"].(map[string]interface{})
		if !ok {
			limits = make(map[string]interface{})
			resources["limits"] = limits
		}
		limits["nvidia.com/gpu"] = fmt.Sprintf("%d", project.GPUQuota)

		env, ok := container["env"].([]interface{})
		if !ok {
			env = make([]interface{}, 0)
		}

		if project.MPSMemory > 0 {
			memoryBytes := int64(project.MPSMemory) * 1024 * 1024
			env = append(env, map[string]interface{}{
				"name":  "CUDA_MPS_PINNED_DEVICE_MEM_LIMIT",
				"value": fmt.Sprintf("%d", memoryBytes),
			})
		}

		container["env"] = env
	}

	return nil
}

// DeleteInstance removes the deployed resources for the calling user.
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

	// Format Namespace for deletion
	safeUsername := k8s.ToSafeK8sName(claims.Username)
	ns := k8s.FormatNamespaceName(configfile.ProjectID, safeUsername)

	for _, val := range data {
		if err := k8s.DeleteByJson(val.ParsedYAML, ns); err != nil {
			return err
		}
	}
	return nil
}

// DeleteConfigFileInstance removes resources for ALL users in the project (Admin/Cleanup).
func (s *ConfigFileService) DeleteConfigFileInstance(id uint) error {
	configfile, err := s.Repos.ConfigFile.GetConfigFileByID(id)
	if err != nil {
		return err
	}

	resources, err := s.Repos.Resource.ListResourcesByConfigFileID(id)
	if err != nil {
		return err
	}

	users, err := s.Repos.User.ListUsersByProjectID(configfile.ProjectID)
	if err != nil {
		return err
	}

	for _, user := range users {
		safeUsername := k8s.ToSafeK8sName(user.Username)
		ns := k8s.FormatNamespaceName(configfile.ProjectID, safeUsername)
		for _, res := range resources {
			// Best effort deletion
			if err := k8s.DeleteByJson(res.ParsedYAML, ns); err != nil {
				fmt.Printf("[Warning] Failed to delete instance for user %s: %v\n", user.Username, err)
			}
		}
	}

	return nil
}

// deleteConfigFileInstance is a private helper for updating config files
func (s *ConfigFileService) deleteConfigFileInstance(configfile *configfile.ConfigFile) ([]resource.Resource, error) {
	resources, err := s.Repos.Resource.ListResourcesByConfigFileID(configfile.CFID)
	if err != nil {
		return nil, err
	}

	users, err := s.Repos.User.ListUsersByProjectID(configfile.ProjectID)
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		safeUsername := k8s.ToSafeK8sName(user.Username)
		ns := k8s.FormatNamespaceName(configfile.ProjectID, safeUsername)
		for _, res := range resources {
			if err := k8s.DeleteByJson(res.ParsedYAML, ns); err != nil {
				return nil, err
			}
		}
	}

	return resources, nil
}

// injectGeneralPodConfig injects general Pod infrastructure settings (permissions and base paths).
func (s *ConfigFileService) injectGeneralPodConfig(jsonBytes []byte) ([]byte, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &obj); err != nil {
		return nil, err
	}

	podSpecs := findPodSpecs(obj)
	if len(podSpecs) == 0 {
		return jsonBytes, nil
	}

	// ================= Configuration Section =================
	const targetUID int64 = 0
	const targetGID int64 = 0
	// =========================================================

	for _, podSpec := range podSpecs {
		// 1. SecurityContext: Resolves PVC write permissions
		secContext, ok := podSpec["securityContext"].(map[string]interface{})
		if !ok {
			secContext = make(map[string]interface{})
			podSpec["securityContext"] = secContext
		}

		secContext["runAsUser"] = targetUID
		secContext["runAsGroup"] = targetGID

		// If volumes are mounted, inject fsGroup to ensure file ownership.
		if _, hasVolumes := podSpec["volumes"]; hasVolumes {
			secContext["fsGroup"] = targetUID
			secContext["fsGroupChangePolicy"] = "OnRootMismatch"
		}
	}

	return json.Marshal(obj)
}

// --- Helper Functions ---

func findPodSpecs(obj map[string]interface{}) []map[string]interface{} {
	var results []map[string]interface{}

	if _, hasContainers := obj["containers"]; hasContainers {
		results = append(results, obj)
		return results
	}

	for key, value := range obj {
		if subMap, ok := value.(map[string]interface{}); ok {
			if key == "spec" || key == "template" || key == "jobTemplate" {
				results = append(results, findPodSpecs(subMap)...)
			}
		}
	}
	return results
}

func getContainers(podSpec map[string]interface{}) []interface{} {
	if containers, ok := podSpec["containers"].([]interface{}); ok {
		return containers
	}
	return []interface{}{}
}
