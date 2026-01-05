package mps

const (
	MPSUnitsPerGPU = 10 // 1 dedicated GPU = 10 MPS units
	// GPU quota is now in integer units (system auto-injects CUDA_MPS_ACTIVE_THREAD_PERCENTAGE)
)

// ConvertGPUToMPS converts dedicated GPU count to MPS units
func ConvertGPUToMPS(gpuCount int) int {
	return gpuCount * MPSUnitsPerGPU
}

// ConvertMPSToGPU converts MPS units to equivalent GPU count
func ConvertMPSToGPU(mpsUnits int) int {
	gpus := mpsUnits / MPSUnitsPerGPU
	if mpsUnits%MPSUnitsPerGPU > 0 {
		gpus++ // Round up to ensure sufficient resources
	}
	return gpus
}

// ValidateGPUQuota validates if GPU quota is positive (system will auto-inject CUDA_MPS_ACTIVE_THREAD_PERCENTAGE)
func ValidateGPUQuota(quota int) bool {
	return quota > 0
}

// CalculateMPSUsage calculates total MPS units from a list of requests
func CalculateMPSUsage(mpsRequests []int) int {
	total := 0
	for _, req := range mpsRequests {
		total += req
	}
	return total
}

// IsWithinQuota checks if requested MPS is within available quota
func IsWithinQuota(requested, available int) bool {
	return requested <= available
}
