package application

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/internal/config"
	"github.com/linskybing/platform-go/internal/domain/configfile"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/pkg/k8s"
	"github.com/linskybing/platform-go/pkg/types"
	"github.com/linskybing/platform-go/pkg/utils"
	"gorm.io/datatypes"
)

// CreateInstance deploys resources to Kubernetes with a high-performance pipeline.
func (s *ConfigFileService) CreateInstance(c *gin.Context, id uint) error {
	// 1. Fetch Data
	resources, err := s.Repos.Resource.ListResourcesByConfigFileID(id)
	if err != nil {
		return err
	}
	cf, err := s.Repos.ConfigFile.GetConfigFileByID(id)
	if err != nil {
		return err
	}

	// 2. Prepare Context (Namespace, Project, Claims)
	ns, proj, claims, err := s.prepareNamespaceAndProject(c, cf)
	if err != nil {
		return err
	}

	// 3. Determine Deployment Strategy (Job-only vs Standard)
	// isJobOnly := s.configFileIsAllJobs(resources)

	// 4. Prepare Variables & Volumes
	var templateValues map[string]string
	var shouldEnforceRO bool
	var projectPVCName string

	// Standard Deployment: Bind Volumes & Check Permissions
	userPvc, projPvc := s.bindProjectAndUserVolumes(ns, proj, claims)
	shouldEnforceRO, err = s.determineReadOnlyEnforcement(claims, proj)
	if err != nil {
		return err
	}
	projectPVCName = projPvc
	templateValues = s.buildTemplateValues(cf, ns, userPvc, projPvc, claims)

	// 5. Processing Pipeline (The most compute-intensive part)
	// We use pre-allocation to avoid slice resizing overhead
	processedResources := make([][]byte, 0, len(resources))

	for _, res := range resources {
		// A. Template Replacement (String Level)
		jsonStr := string(res.ParsedYAML)
		replacedJSON, err := utils.ReplacePlaceholdersInJSON(jsonStr, templateValues)
		if err != nil {
			return fmt.Errorf("failed to replace placeholders for resource %s: %w", res.Name, err)
		}

		// B. Unmarshal ONCE (Performance Key)
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(replacedJSON), &obj); err != nil {
			return fmt.Errorf("failed to unmarshal resource %s: %w", res.Name, err)
		}

		// C. Apply Patches (In-Memory Map Manipulation)
		//    All business logic validation and injection happens here without re-marshaling.
		ctx := &PatchContext{
			ProjectID:       cf.ProjectID,
			Project:         proj,
			UserIsAdmin:     claims.IsAdmin,
			ShouldEnforceRO: shouldEnforceRO,
			ProjectPVC:      projectPVCName,
		}

		if err := s.applyResourcePatches(obj, ctx); err != nil {
			return fmt.Errorf("failed to patch resource %s: %w", res.Name, err)
		}

		// D. Marshal ONCE
		finalBytes, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("failed to marshal final resource %s: %w", res.Name, err)
		}

		processedResources = append(processedResources, finalBytes)
	}

	// 6. Apply to Kubernetes
	log.Printf("Deploying %d resources to namespace %s", len(processedResources), ns)
	for _, jsonBytes := range processedResources {
		if err := k8s.CreateByJson(datatypes.JSON(jsonBytes), ns); err != nil {
			return fmt.Errorf("failed to create resource in k8s: %w", err)
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

	safeUsername := k8s.ToSafeK8sName(claims.Username)
	ns := k8s.FormatNamespaceName(configfile.ProjectID, safeUsername)

	for _, val := range data {
		if err := k8s.DeleteByJson(val.ParsedYAML, ns); err != nil {
			// Continue deleting other resources even if one fails
			fmt.Printf("[Error] Failed to delete resource %s: %v\n", val.Name, err)
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

	users, err := s.Repos.User.ListUsersByProjectID(configfile.ProjectID)
	if err != nil {
		return err
	}

	for _, user := range users {
		safeUsername := k8s.ToSafeK8sName(user.Username)
		ns := k8s.FormatNamespaceName(configfile.ProjectID, safeUsername)
		for _, res := range resources {
			if err := k8s.DeleteByJson(res.ParsedYAML, ns); err != nil {
				fmt.Printf("[Warning] Failed to delete instance for user %s: %v\n", user.Username, err)
			}
		}
	}

	return nil
}

// --- Helpers for Deployment ---

func (s *ConfigFileService) prepareNamespaceAndProject(c *gin.Context, cf *configfile.ConfigFile) (string, project.Project, *types.Claims, error) {
	claims, _ := c.MustGet("claims").(*types.Claims)
	safeUsername := k8s.ToSafeK8sName(claims.Username)
	targetNs := k8s.FormatNamespaceName(cf.ProjectID, safeUsername)

	if err := k8s.EnsureNamespaceExists(targetNs); err != nil {
		return "", project.Project{}, nil, fmt.Errorf("failed to ensure namespace %s: %w", targetNs, err)
	}

	p, err := s.Repos.Project.GetProjectByID(cf.ProjectID)
	if err != nil {
		return "", project.Project{}, nil, err
	}
	return targetNs, p, claims, nil
}

func (s *ConfigFileService) bindProjectAndUserVolumes(targetNs string, project project.Project, claims *types.Claims) (string, string) {
	safeUsername := k8s.ToSafeK8sName(claims.Username)
	userStorageNs := fmt.Sprintf(config.UserStorageNs, safeUsername)
	userPvcName := fmt.Sprintf(config.UserStoragePVC, safeUsername)
	projectStorageNs := k8s.GenerateSafeResourceName("project", project.ProjectName, project.PID)
	projectPvcName := fmt.Sprintf("project-%d-disk", project.PID)

	targetUserPvcName := userPvcName
	if err := k8s.MountExistingVolumeToProject(userStorageNs, userPvcName, targetNs, targetUserPvcName); err != nil {
		fmt.Printf("[Warning] Failed to bind user volume: %v\n", err)
	}

	targetProjectPvcName := projectPvcName
	if err := k8s.MountExistingVolumeToProject(projectStorageNs, projectPvcName, targetNs, targetProjectPvcName); err != nil {
		fmt.Printf("[Warning] Failed to bind project volume: %v\n", err)
	}

	return targetUserPvcName, targetProjectPvcName
}

func (s *ConfigFileService) determineReadOnlyEnforcement(claims *types.Claims, project project.Project) (bool, error) {
	if claims.IsAdmin {
		return false, nil
	}
	ug, err := s.Repos.UserGroup.GetUserGroup(claims.UserID, project.GID)
	if err != nil {
		// If user is not in group, default to safe (Enforce RO) or error?
		// Assuming error means access denied usually, but let's be strict.
		return true, err
	}
	// Only managers and admins get write access
	return ug.Role != "manager" && ug.Role != "admin", nil
}

func (s *ConfigFileService) buildTemplateValues(cf *configfile.ConfigFile, namespace, userPvc, projectPvc string, claims *types.Claims) map[string]string {
	return map[string]string{
		"username":         k8s.ToSafeK8sName(claims.Username),
		"originalUsername": claims.Username,
		"safeUsername":     k8s.ToSafeK8sName(claims.Username),
		"namespace":        namespace,
		"projectId":        fmt.Sprintf("%d", cf.ProjectID),
		"userVolume":       userPvc,
		"projectVolume":    projectPvc,
	}
}

// func (s *ConfigFileService) buildJobTemplateValues(cf *configfile.ConfigFile, namespace string, claims *types.Claims) map[string]string {
// 	safeUsername := k8s.ToSafeK8sName(claims.Username)
// 	return map[string]string{
// 		"username":         safeUsername,
// 		"originalUsername": claims.Username,
// 		"safeUsername":     safeUsername,
// 		"namespace":        namespace,
// 		"projectId":        fmt.Sprintf("%d", cf.ProjectID),
// 	}
// }

// func (s *ConfigFileService) configFileIsAllJobs(data []resource.Resource) bool {
// 	if len(data) == 0 {
// 		return false
// 	}
// 	for _, r := range data {
// 		if !strings.EqualFold(string(r.Type), string(resource.ResourceJob)) {
// 			return false
// 		}
// 	}
// 	return true
// }
