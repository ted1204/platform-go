package mps

import "fmt"

// MPSConfig represents MPS configuration for a container
type MPSConfig struct {
	GPUQuota      int // GPU quota in integer units (system auto-injects CUDA_MPS_ACTIVE_THREAD_PERCENTAGE)
	MemoryLimitMB int // Memory limit in MB (0 = no limit)
}

// Validate validates the MPS configuration
func (c *MPSConfig) Validate() error {
	if c.GPUQuota < 0 {
		return nil
	}
	if c.MemoryLimitMB < 0 {
		return nil
	}
	return nil
}

// ToEnvVars converts MPS config to environment variables for containers
// Note: CUDA_MPS_ACTIVE_THREAD_PERCENTAGE is auto-injected by the system
func (c *MPSConfig) ToEnvVars() map[string]string {
	env := make(map[string]string)
	if c.GPUQuota > 0 {
		env["GPU_QUOTA"] = fmt.Sprintf("%d", c.GPUQuota)
	}
	if c.MemoryLimitMB > 0 {
		// Convert MB to bytes for CUDA_MPS_PINNED_DEVICE_MEM_LIMIT
		memoryBytes := int64(c.MemoryLimitMB) * 1024 * 1024
		env["CUDA_MPS_PINNED_DEVICE_MEM_LIMIT"] = fmt.Sprintf("%d", memoryBytes)
	}
	return env
}
