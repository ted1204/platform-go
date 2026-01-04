package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gorm.io/datatypes"
)

var (
	ErrConfigFileNotFound  = errors.New("config file not found")
	ErrYAMLParsingFailed   = errors.New("YAML parsing failed")
	ErrNoValidYAMLDocument = errors.New("no valid YAML documents found")
	ErrUploadYAMLFailed    = errors.New("failed to upload YAML file")
)

type ConfigFileService struct {
	Repos *repository.Repos
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
		Repos: repos,
	}
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

	var resources []*resource.Resource
	for i, doc := range yamlArray {
		jsonContent, err := utils.YAMLToJSON(doc)
		if err != nil {
			return nil, fmt.Errorf("failed to convert YAML to JSON for document %d: %w", i+1, err)
		}

		gvk, name, err := utils.ValidateK8sJSON(jsonContent)
		if err != nil {
			return nil, fmt.Errorf("failed to validate YAML document %d: %w", i+1, err)
		}

		resources = append(resources, &resource.Resource{
			Type:       resource.ResourceType(normalizeResourceKind(gvk.Kind)),
			Name:       name,
			ParsedYAML: datatypes.JSON([]byte(jsonContent)),
		})
	}

	createdCF := &configfile.ConfigFile{
		Filename:  cf.Filename,
		Content:   cf.RawYaml,
		ProjectID: cf.ProjectID,
	}
	if err := s.Repos.ConfigFile.CreateConfigFile(createdCF); err != nil {
		return nil, err
	}
	go utils.LogAuditWithConsole(c, "create", "config_file", fmt.Sprintf("cf_id=%d", createdCF.CFID), nil, *createdCF, "", s.Repos.Audit)

	for _, res := range resources {
		res.CFID = createdCF.CFID
		if err := s.Repos.Resource.CreateResource(res); err != nil {
			return nil, fmt.Errorf("failed to create resource %s/%s: %w", res.Type, res.Name, err)
		}
		go utils.LogAuditWithConsole(c, "create", "resource", fmt.Sprintf("r_id=%d", res.RID), nil, *res, "", s.Repos.Audit)
	}

	return createdCF, nil
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
		jsonContent, err := utils.YAMLToJSON(doc)
		if err != nil {
			return fmt.Errorf("failed to convert YAML to JSON for document %d: %w", i+1, err)
		}

		gvk, name, err := utils.ValidateK8sJSON(jsonContent)
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
			fmt.Printf("update resource for ccc document %d: %s", i+1, name)
			if err := s.Repos.Resource.CreateResource(res); err != nil {
				return fmt.Errorf("failed to create resource for document %d: %w", i+1, err)
			}
			go utils.LogAuditWithConsole(c, "create", "resource", fmt.Sprintf("r_id=%d", res.RID), nil, *res, "", s.Repos.Audit)
		} else {
			usedKeys[name] = true
			oldTarget := val
			val.Name = name
			val.ParsedYAML = datatypes.JSON([]byte(jsonContent))
			fmt.Printf("update resource for document %d: %s", i+1, name)
			if err := s.Repos.Resource.UpdateResource(&val); err != nil {
				return fmt.Errorf("failed to update resource for document %d: %w", i+1, err)
			}
			go utils.LogAuditWithConsole(c, "update", "resource", fmt.Sprintf("r_id=%d", val.RID), oldTarget, val, "", s.Repos.Audit)
		}
	}

	for key, val := range usedKeys {
		if !val {
			if err := s.Repos.Resource.DeleteResource(resourceMap[key].RID); err != nil {
				return fmt.Errorf("failed to delete unused resource %s: %w", key, err)
			}
			go utils.LogAuditWithConsole(c, "delete", "resource", fmt.Sprintf("r_id=%d", resourceMap[key].RID), resourceMap[key], nil, "", s.Repos.Audit)
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

	go utils.LogAuditWithConsole(c, "update", "config_file", fmt.Sprintf("cf_id=%d", existing.CFID), oldCF, *existing, "", s.Repos.Audit)

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

	go utils.LogAuditWithConsole(c, "delete", "config_file", fmt.Sprintf("cf_id=%d", cf.CFID), *cf, nil, "", s.Repos.Audit)
	return nil
}

func (s *ConfigFileService) ListConfigFilesByProjectID(projectID uint) ([]configfile.ConfigFile, error) {
	return s.Repos.ConfigFile.GetConfigFilesByProjectID(projectID)
}

func (s *ConfigFileService) CreateInstance(c *gin.Context, id uint) error {
	// 1. Retrieve config file data and raw YAML resources
	data, err := s.Repos.Resource.ListResourcesByConfigFileID(id)
	if err != nil {
		return err
	}
	configfile, err := s.Repos.ConfigFile.GetConfigFileByID(id)
	if err != nil {
		return err
	}

	// 2. Get user claims from context
	claims, _ := c.MustGet("claims").(*types.Claims)

	// -------------------------------------------------------------------------
	// CORE FIX 1: Sanitize Username for Kubernetes
	// -------------------------------------------------------------------------
	// Convert the raw username (e.g., "John_Doe") to a valid Kubernetes resource name
	// (e.g., "john-doe") to comply with RFC 1123 standards.
	// This prevents errors when creating Namespaces or Services.
	safeUsername := utils.ToSafeK8sName(claims.Username)

	// Generate the target project namespace using the sanitized username.
	// Example: "proj-1-john-doe"
	ns := utils.FormatNamespaceName(configfile.ProjectID, safeUsername)

	// Fetch project info early for namespace derivation and permission checks
	project, err := s.Repos.Project.GetProjectByID(configfile.ProjectID)
	if err != nil {
		return err
	}

	// Calculate the user's storage namespace where the NFS service resides.
	// Example: "user-john-doe-storage"
	userStorageNamespace := fmt.Sprintf("user-%s-storage", safeUsername)

	// -------------------------------------------------------------------------
	// CORE FIX 2: Resolve NFS Service IP (Bypassing DNS)
	// -------------------------------------------------------------------------
	// Default to the internal DNS name. This acts as a fallback if the lookup fails
	// or if the service is in a different cluster.
	nfsServerAddress := fmt.Sprintf("%s.%s.svc.cluster.local", config.PersonalStorageServiceName, userStorageNamespace)

	// Attempt to resolve the ClusterIP of the NFS service directly.
	// This is critical for environments like Docker Desktop where internal K8s DNS
	// resolution might be flaky or unsupported from the host/node level.
	if k8s.Clientset != nil {
		// We use context.TODO() here, or you can inherit context from gin if available.
		svc, err := k8s.Clientset.CoreV1().Services(userStorageNamespace).Get(
			context.TODO(),
			config.PersonalStorageServiceName, // The configured name of your NFS service
			metav1.GetOptions{},
		)

		// If the service is found and has a valid ClusterIP, use it.
		if err == nil && svc.Spec.ClusterIP != "" {
			nfsServerAddress = svc.Spec.ClusterIP
		}
	}

	// -------------------------------------------------------------------------
	// Project Storage: Resolve Project NFS Service IP
	// -------------------------------------------------------------------------
	// For project storage, the NFS service resides in the project namespace.
	// Use the same namespace scheme as project resources (e.g., GenerateSafeResourceName).
	projectNamespace := utils.GenerateSafeResourceName("project", project.ProjectName, project.PID)
	projectNfsServerAddress := ""

	// Prefer direct ClusterIP lookup (same pattern as personal NFS)
	if k8s.Clientset != nil {
		svc, err := k8s.Clientset.CoreV1().Services(projectNamespace).Get(
			context.TODO(),
			config.ProjectNfsServiceName, // The configured name of your project NFS service
			metav1.GetOptions{},
		)

		if err == nil && svc.Spec.ClusterIP != "" {
			projectNfsServerAddress = svc.Spec.ClusterIP
		}
	}

	// Fallback to cluster DNS name if ClusterIP not resolved
	if projectNfsServerAddress == "" {
		projectNfsServerAddress = fmt.Sprintf("%s.%s.svc.cluster.local", config.ProjectNfsServiceName, projectNamespace)
	}

	// 3. Check Project Permissions & ReadOnly Enforcement
	shouldEnforceRO := false
	if !claims.IsAdmin {
		ug, err := s.Repos.UserGroup.GetUserGroup(claims.UserID, project.GID)
		if err != nil {
			return err
		}
		// Only enforce readonly if user is NOT manager/admin/owner
		// Manager and above can write to project storage
		if ug.Role != "manager" && ug.Role != "admin" && ug.Role != "owner" {
			shouldEnforceRO = true
		}
	}

	// -------------------------------------------------------------------------
	// Prepare Template Variables
	// -------------------------------------------------------------------------
	// Inject the resolved variables into the map. These will replace placeholders
	// like {{username}} or {{nfsServer}} in the YAML files.
	templateValues := map[string]string{
		// "username" is now the safe version to ensure resource names in YAML are valid.
		"username": safeUsername,

		// Provide the original username in case it's needed for labels/annotations.
		"originalUsername": claims.Username,

		// Explicit safe username variable.
		"safeUsername": safeUsername,

		// The target namespace for deployment.
		"namespace": ns,

		// The resolved NFS server address (IP or DNS) for personal storage.
		"nfsServer": nfsServerAddress,

		// The resolved NFS server address (IP or DNS) for project storage.
		"projectNfsServer": projectNfsServerAddress,

		// The namespace where user storage is located.
		"userStorageNamespace": userStorageNamespace,

		// Project ID as a string.
		"projectId": fmt.Sprintf("%d", configfile.ProjectID),
	}

	// 4. Iterate and Create Resources
	for _, val := range data {
		// Convert YAML content to string
		jsonStr := string(val.ParsedYAML)

		// Perform variable replacement
		replacedJSON, err := utils.ReplacePlaceholdersInJSON(jsonStr, templateValues)
		if err != nil {
			return fmt.Errorf("failed to replace placeholders: %w", err)
		}

		jsonBytes := []byte(replacedJSON)

		// // Normalize NFS servers: if path points to project storage (/srv/...), force project NFS server
		// jsonBytes, err = s.rewriteNfsServers(jsonBytes, nfsServerAddress, projectNfsServerAddress)
		// if err != nil {
		// 	return err
		// }

		// Apply ReadOnly restrictions if necessary
		if shouldEnforceRO {
			jsonBytes, err = s.enforceReadOnly(jsonBytes)
			if err != nil {
				return err
			}
		}

		// Inject GPU configurations based on project quota
		jsonBytes, err = s.InjectGPUAnnotations(jsonBytes, project)
		if err != nil {
			return err
		}

		// Finally, apply the resource to the Kubernetes cluster
		if err := utils.CreateByJson(datatypes.JSON(jsonBytes), ns); err != nil {
			return err
		}
	}
	return nil
}

func (s *ConfigFileService) enforceReadOnly(jsonBytes []byte) ([]byte, error) {
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
	}

	if podSpec == nil {
		return jsonBytes, nil
	}

	// Find volumes and identify which are PVCs vs NFS
	volumes, ok := podSpec["volumes"].([]interface{})
	if !ok {
		return jsonBytes, nil
	}

	// Map volume name to PVC claim name
	pvcVolumes := make(map[string]string)
	for _, v := range volumes {
		if vol, ok := v.(map[string]interface{}); ok {
			if name, ok := vol["name"].(string); ok {
				// Check if it's a PVC volume
				if pvc, ok := vol["persistentVolumeClaim"].(map[string]interface{}); ok {
					if claimName, ok := pvc["claimName"].(string); ok {
						pvcVolumes[name] = claimName
					}
				}
			}
		}
	}

	// Iterate containers and modify volumeMounts
	containers, ok := podSpec["containers"].([]interface{})
	if !ok {
		return jsonBytes, nil
	}

	for _, c := range containers {
		if container, ok := c.(map[string]interface{}); ok {
			if mounts, ok := container["volumeMounts"].([]interface{}); ok {
				for _, m := range mounts {
					if mount, ok := m.(map[string]interface{}); ok {
						if volName, ok := mount["name"].(string); ok {
							// Only set readonly for PVC volumes (project storage)
							// NFS volumes (user storage) remain writable
							if claimName, isPVC := pvcVolumes[volName]; isPVC {
								// Don't set readonly for default user storage PVC
								if claimName != config.DefaultStorageName {
									mount["readOnly"] = true
								}
							}
							// NFS volumes are never set to readonly here
						}
					}
				}
			}
		}
	}

	return json.Marshal(obj)
}

// Helper to inject GPU annotations into Pod spec
func (s *ConfigFileService) InjectGPUAnnotations(jsonBytes []byte, project project.Project) ([]byte, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &obj); err != nil {
		return nil, err
	}

	metadata, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		metadata = make(map[string]interface{})
		obj["metadata"] = metadata
	}

	annotations, ok := metadata["annotations"].(map[string]interface{})
	if !ok {
		annotations = make(map[string]interface{})
		metadata["annotations"] = annotations
	}

	// Inject MPS Annotations
	if project.MPSLimit > 0 {
		annotations["mps.nvidia.com/threads"] = fmt.Sprintf("%d", project.MPSLimit)
	}
	if project.MPSMemory > 0 {
		annotations["mps.nvidia.com/vram"] = fmt.Sprintf("%dM", project.MPSMemory)
	}

	return json.Marshal(obj)
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

func (s *ConfigFileService) deleteConfigFileInstance(configfile *configfile.ConfigFile) ([]resource.Resource, error) {
	resources, err := s.Repos.Resource.ListResourcesByConfigFileID(configfile.CFID)
	if err != nil {
		return nil, err
	}

	users, err := s.Repos.View.ListUsersByProjectID(configfile.ProjectID)
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		ns := utils.FormatNamespaceName(configfile.ProjectID, user.Username)
		for _, res := range resources {
			if err := utils.DeleteByJson(res.ParsedYAML, ns); err != nil {
				return nil, err
			}
		}
	}

	return resources, nil
}
