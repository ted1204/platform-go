# GPU MPS Memory Limit Implementation

## Overview

This document describes the implementation of GPU MPS (Multi-Process Service) memory limit validation and injection for Kubernetes workloads.

## Architecture & Design

### Key Principles

1. **Conditional Validation**: GPU MPS configuration is only validated and injected when workloads explicitly request GPU resources (`nvidia.com/gpu`)
2. **Dual-Validation Pattern**: 
   - First validation at config file creation (checks project MPS config)
   - Second validation at instance deployment (validates container specs and injects env vars)
3. **Non-Intrusive**: Non-GPU workloads are not affected by MPS configuration

### Activation Criteria

GPU MPS configuration is activated ONLY when:
- Container has `resources.requests.nvidia.com/gpu` defined
- Project has valid MPS configuration (MPSLimit > 0 AND MPSMemory > 0)
- MPS limits are within acceptable ranges (thread: 0-100%, memory: >= 512MB)

## Implementation Details

### Backend Changes

#### 1. MPS Configuration Package (`pkg/mps/config.go`)

**Changes:**
- Updated `MPSConfig.ToEnvVars()` to properly convert memory limits to CUDA environment variables
- Memory limit is converted from MB to bytes for `CUDA_MPS_PINNED_DEVICE_MEM_LIMIT`
- Thread percentage is converted to string for `CUDA_MPS_ACTIVE_THREAD_PERCENTAGE`

**Environment Variables Injected:**
```
CUDA_MPS_PINNED_DEVICE_MEM_LIMIT = MemoryLimitMB * 1024 * 1024 (bytes)
CUDA_MPS_ACTIVE_THREAD_PERCENTAGE = ThreadPercentage (0-100)
```

#### 2. ConfigFile Service (`internal/application/configfile.go`)

**New Methods:**

1. **`ValidateAndInjectGPUConfig()`**
   - Replaces old `InjectGPUAnnotations()` method
   - Checks if container requests GPU resources
   - Validates project MPS configuration
   - Injects MPS configuration into pod specs
   - Returns unchanged resource if no GPU request

2. **`containerHasGPURequest()`**
   - Helper method to detect `nvidia.com/gpu` requests
   - Iterates through all containers in pod spec
   - Returns boolean indicating GPU usage

3. **`validateProjectMPSConfig()`**
   - First validation point
    - Ensures GPUQuota is > 0
    - Validates optional MPSMemory >= 512MB (minimum requirement when set)

4. **`injectMPSConfig()`**
   - Second validation/injection point
   - Injects resource limits:
       - `nvidia.com/gpu`: GPU quota (integer units)
    - Injects CUDA environment variables
       - `GPU_QUOTA`
       - `CUDA_MPS_PINNED_DEVICE_MEM_LIMIT` (only when MPSMemory is set)
    - `CUDA_MPS_ACTIVE_THREAD_PERCENTAGE` is auto-injected by the system
   - Only runs for containers with GPU requests

#### 3. Swagger Documentation

Updated `CreateInstanceHandler` swagger documentation to clarify:
- MPS validation is conditional on GPU resource requests
- Non-GPU workloads skip validation
- Proper error codes (400 for validation, 500 for internal errors)

### Test Coverage

#### Unit Tests (`internal/application/configfile_service_test.go`)

New test suite `TestValidateAndInjectGPUConfig` with 5 test cases:

1. **GPUConfig_WithoutGPURequest**
   - Verifies non-GPU pods pass through unchanged
   - Ensures no env vars are injected

2. **GPUConfig_WithGPURequest_ValidConfig**
   - Verifies GPU config is properly injected
   - Checks GPU limit and env vars are set

3. **GPUConfig_InvalidGPUQuota**
   - Tests error handling for zero/invalid GPU quota
   - Ensures validation prevents invalid configs

4. **GPUConfig_InvalidMPSMemory**
   - Tests error handling for insufficient memory
   - Ensures minimum 512MB requirement is enforced

5. **GPUConfig_MPSMemoryOptional**
   - Tests optional MPS memory (0 means disabled)
   - Ensures GPU quota still injects envs without memory

#### MPS Package Tests (`pkg/mps/mps_test.go`)

New test suite `TestMPSConfigToEnvVars` with 3 test cases:

1. **ThreadPercentage and MemoryLimit both set**
   - Verifies correct conversion to environment variables
   - Tests byte conversion for memory limits

2. **Only MemoryLimit set**
   - Verifies partial configuration handling
   - Ensures only configured values are injected

3. **No configuration set**
   - Verifies empty env vars for zero config
   - Ensures no unnecessary variables

#### Integration Tests (`test/integration/configfile_handler_test.go`)

New test suite `TestConfigFileGPUMPSConfiguration` with 4 test cases:

1. **GPU request without MPS config - Should fail**
   - Tests error handling during instance creation
   
2. **GPU request with valid MPS config**
   - Tests successful config creation and injection
   
3. **Non-GPU workload ignores MPS config**
   - Verifies non-GPU pods succeed despite MPS
   
4. **Deployment with GPU request**
   - Tests MPS injection for Deployment resources

## Security Considerations

1. **Input Validation**: All MPS limits are validated against acceptable ranges
2. **Type Safety**: Proper type conversion for environment variables (int to string, MB to bytes)
3. **Resource Isolation**: Memory limits prevent MPS processes from consuming excessive GPU memory
4. **Least Privilege**: Configuration is only applied when explicitly needed (GPU requests)
5. **Error Messages**: Clear error messages for validation failures (no information leakage)

## Kubernetes Integration

### Pod Spec Modification

For pods requesting GPU resources, the following is injected:

```yaml
spec:
  containers:
  - name: gpu-container
    resources:
      requests:
        nvidia.com/gpu: "1"
      limits:
            nvidia.com/gpu: "10"
    env:
      - name: GPU_QUOTA
         value: "10"
    - name: CUDA_MPS_PINNED_DEVICE_MEM_LIMIT
      value: "2147483648"  # 2GB in bytes
```

## Database Schema

### Project Table Changes

No schema changes required. Uses existing fields:
- `mps_limit`: Thread percentage limit (0-100)
- `mps_memory`: Memory limit in MB (0 means no limit)

## Testing & Validation

### Run Unit Tests

```bash
cd backend
go test ./internal/application -v -run TestValidateAndInjectGPUConfig
go test ./pkg/mps -v -run TestMPSConfigToEnvVars
```

### Run Integration Tests

```bash
cd backend
go test ./test/integration -v -run TestConfigFileGPUMPSConfiguration
```

### Build Verification

```bash
cd backend
go build ./cmd/api ./cmd/scheduler
```

## Backward Compatibility

- Old `InjectGPUAnnotations()` method removed (was only used internally)
- No API changes for external consumers
- No database migrations required
- Existing non-GPU workloads unaffected

## Future Enhancements

1. Support for MPS compute instance management
2. Dynamic memory allocation based on workload
3. Per-container MPS configuration
4. MPS statistics and monitoring
5. Automatic MPS limit adjustment based on cluster usage
