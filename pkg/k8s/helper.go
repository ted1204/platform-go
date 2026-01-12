package k8s

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
)

func ToSafeK8sName(rawName string) string {
	safeName := strings.ToLower(rawName)

	reg := regexp.MustCompile(`[^a-z0-9]+`)
	safeName = reg.ReplaceAllString(safeName, "-")

	safeName = strings.Trim(safeName, "-")

	multiHyphenReg := regexp.MustCompile(`-+`)
	safeName = multiHyphenReg.ReplaceAllString(safeName, "-")

	if len(safeName) > 63 {
		safeName = safeName[:63]
		safeName = strings.TrimRight(safeName, "-")
	}

	if safeName == "" {
		safeName = "unnamed"
	}

	return safeName
}

// GenerateSafeResourceName generates a unique and K8s-compliant resource name.
// Format: prefix-{sanitized_name}-{short_hash}
// Constraint: Kubernetes names must be max 63 characters, lowercase, alphanumeric, or hyphen.
func GenerateSafeResourceName(prefix string, name string, id uint) string {
	// 1. Sanitize Name: Keep only lowercase alphanumeric characters and hyphens.
	// Replace invalid characters with a hyphen.
	reg := regexp.MustCompile("[^a-z0-9]+")
	safeName := reg.ReplaceAllString(strings.ToLower(name), "-")
	safeName = strings.Trim(safeName, "-") // Remove leading/trailing hyphens

	// 2. Generate Short Hash from ID to ensure uniqueness.
	// Using the ID as a seed ensures that the same Project ID always generates the same namespace name.
	hashInput := fmt.Sprintf("project-%d", id)
	hash := sha256.Sum256([]byte(hashInput))
	shortHash := fmt.Sprintf("%x", hash)[:6] // Take the first 6 characters of the hash

	// 3. Construct the final name.
	// Format: prefix-name-hash
	// We need to ensure the total length does not exceed 63 characters.
	baseName := fmt.Sprintf("%s-%s", prefix, safeName)
	suffix := fmt.Sprintf("-%s", shortHash)

	// Calculate max allowed length for the base name to accommodate the suffix.
	maxLength := 63 - len(suffix)
	if len(baseName) > maxLength {
		baseName = baseName[:maxLength]
		// Ensure we don't end with a hyphen after truncation
		baseName = strings.TrimRight(baseName, "-")
	}

	return baseName + suffix
}
