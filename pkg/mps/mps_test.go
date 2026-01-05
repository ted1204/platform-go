package mps

import (
	"fmt"
	"testing"
)

func TestConvertGPUToMPS(t *testing.T) {
	if got := ConvertGPUToMPS(2); got != 20 {
		t.Fatalf("expected 20 MPS units, got %d", got)
	}
}

func TestConvertMPSToGPU(t *testing.T) {
	if got := ConvertMPSToGPU(15); got != 2 {
		t.Fatalf("expected 2 GPUs for 15 units, got %d", got)
	}
}

func TestValidateGPUQuota(t *testing.T) {
	if !ValidateGPUQuota(100) {
		t.Fatalf("expected quota 100 to be valid")
	}
	if ValidateGPUQuota(-1) {
		t.Fatalf("expected negative quota to be invalid")
	}
	if ValidateGPUQuota(0) {
		t.Fatalf("expected zero quota to be invalid")
	}
}

func TestProjectMPSQuota(t *testing.T) {
	q := &ProjectMPSQuota{TotalMPSUnits: 100, UsedMPSUnits: 40}
	if !q.CanAllocate(50) {
		t.Fatalf("expected to allocate 50 units")
	}
	if q.CanAllocate(70) {
		t.Fatalf("expected 70 allocation to fail")
	}
	if q.UsagePercent() != 40 {
		t.Fatalf("expected usage percent 40, got %f", q.UsagePercent())
	}
}

func TestMPSConfigToEnvVars(t *testing.T) {
	t.Run("GPUQuota and MemoryLimit both set", func(t *testing.T) {
		cfg := &MPSConfig{
			GPUQuota:      80,
			MemoryLimitMB: 2048,
		}

		env := cfg.ToEnvVars()

		// Check GPU quota env var
		if quotaVal, ok := env["GPU_QUOTA"]; !ok {
			t.Fatalf("expected GPU_QUOTA to be set")
		} else if quotaVal != "80" {
			t.Fatalf("expected GPU quota 80, got %s", quotaVal)
		}

		// Check memory limit env var (should be in bytes)
		if memVal, ok := env["CUDA_MPS_PINNED_DEVICE_MEM_LIMIT"]; !ok {
			t.Fatalf("expected CUDA_MPS_PINNED_DEVICE_MEM_LIMIT to be set")
		} else {
			expectedBytes := int64(2048) * 1024 * 1024
			expectedStr := fmt.Sprintf("%d", expectedBytes)
			if memVal != expectedStr {
				t.Fatalf("expected memory limit %s bytes, got %s", expectedStr, memVal)
			}
		}
	})

	t.Run("Only MemoryLimit set", func(t *testing.T) {
		cfg := &MPSConfig{
			GPUQuota:      0,
			MemoryLimitMB: 1024,
		}

		env := cfg.ToEnvVars()

		// Should not have GPU quota env var when 0
		if _, ok := env["GPU_QUOTA"]; ok {
			t.Fatalf("should not set GPU quota when 0")
		}

		// Should have memory limit env var
		if _, ok := env["CUDA_MPS_PINNED_DEVICE_MEM_LIMIT"]; !ok {
			t.Fatalf("expected CUDA_MPS_PINNED_DEVICE_MEM_LIMIT to be set")
		}
	})

	t.Run("No configuration set", func(t *testing.T) {
		cfg := &MPSConfig{
			GPUQuota:      0,
			MemoryLimitMB: 0,
		}

		env := cfg.ToEnvVars()

		// Should be empty
		if len(env) != 0 {
			t.Fatalf("expected empty env vars, got %v", env)
		}
	})
}
