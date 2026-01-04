# Integration Testing Quick Start Guide

## 5-Minute Quick Start

### 1. Prepare Environment

```bash
# Ensure PostgreSQL is running
pg_isready

# Create test database
createdb platform_test

# Ensure K8s cluster is accessible
kubectl cluster-info
```

### 2. Configure Test Environment

```bash
# Copy test configuration
cp test/integration/.env.test .env

# Or manually set environment variables
export TEST_DB_HOST=localhost
export TEST_DB_NAME=platform_test
```

### 3. Run Tests

```bash
# Run all integration tests
make test-integration

# Or use go test directly
go test -v -timeout 30m ./test/integration/...
```

## Test Coverage

### API Handler Tests

Each handler includes the following tests:

1. **Functional Tests**
   - CRUD operations
   - Business logic validation
   - Data associations

2. **Boundary Condition Tests**
   - Null value handling
   - Extreme value handling
   - Edge value testing
   - Concurrent access

3. **Permission Management Tests**
   - Admin permissions
   - Manager permissions
   - User permissions
   - Unauthorized access protection

4. **K8s Operation Validation**
   - Resource creation confirmation
   - Resource status check
   - Resource deletion verification
   - Namespace management

### Test Data Flow

```
TestMain (initialization)
  |- Create test database
  |- Initialize K8s client
  |- Create test users (admin, manager, user)
  |- Create test groups and projects
  \- Generate JWT tokens
      |
Test cases run
  |- Use HTTPClient to send requests
  |- Verify response status and data
  |- Use K8sValidator to verify K8s resources
  \- Clean up test-created resources
      |
TestMain (cleanup)
  |- Delete test namespaces
  |- Clean database
  \- Release resources
```

## Main Test Files

| File | Description | Test Count |
|------|-------------|------------|
| `setup_test.go` | Test framework and initialization | - |
| `http_client.go` | HTTP request tools | - |
| `k8s_validator.go` | K8s validation tools | - |
| `helpers.go` | Test helper utilities | - |
| `project_handler_test.go` | Project management tests | 15+ |
| `configfile_handler_test.go` | Configuration file tests | 12+ |
| `user_handler_test.go` | User management tests | 20+ |
| `group_handler_test.go` | Group management tests | 25+ |
| `k8s_handler_test.go` | K8s operation tests | 20+ |
| `gpu_form_handler_test.go` | GPU/Form tests | 15+ |

**Total: 100+ test cases**

## Permission Test Matrix

| Operation | Admin | Manager | User | Anonymous |
|-----------|-------|---------|------|----------|
| Create Project | Yes | No | No | No |
| Delete Project | Yes | No | No | No |
| Update Project | Yes | Yes | No | No |
| View Project | Yes | Yes | Yes | No |
| Create Config | Yes | Yes | No | No |
| Delete Config | Yes | Yes | No | No |
| Create Instance | Yes | Yes | Yes | No |
| Delete Instance | Yes | Yes | Yes | No |
| Create Group | Yes | No | No | No |
| Manage UserGroup | Yes | No | No | No |
| K8s PVC Create | Yes | No | No | No |
| GPU Request | Yes | Yes | Yes | No |
| GPU Approval | Yes | No | No | No |

## K8s Resource Validation Examples

### PVC Creation Validation

```go
// 1. Create PVC via API
resp, err := client.POST("/k8s/pvc", pvcDTO)
require.NoError(t, err)
assert.Equal(t, http.StatusOK, resp.StatusCode)

// 2. Verify PVC exists in K8s
time.Sleep(2 * time.Second)
exists, err := k8sValidator.PVCExists(namespace, "test-pvc")
require.NoError(t, err)
assert.True(t, exists)

// 3. Verify PVC properties
pvc, err := k8sValidator.GetPVC(namespace, "test-pvc")
require.NoError(t, err)
assert.Equal(t, "1Gi", pvc.Spec.Resources.Requests["storage"])
```

### Deployment Creation Validation

```go
// 1. Create instance via API
resp, err := client.POST("/instance/1", nil)
require.NoError(t, err)

// 2. Wait for Deployment to be ready
err = k8sValidator.WaitForDeploymentReady(namespace, deploymentName, 60*time.Second)
require.NoError(t, err)

// 3. Verify Pod is running
pods, err := k8sValidator.ListPods(namespace, "app=my-app")
require.NoError(t, err)
assert.GreaterOrEqual(t, len(pods.Items), 1)
```

## Boundary Condition Test Examples

### Input Validation

```go
tests := []struct {
    name  string
    input map[string]interface{}
    want  int
}{
    {
        name: "Empty name",
        input: map[string]interface{}{"name": ""},
        want: http.StatusBadRequest,
    },
    {
        name: "Overly long name",
        input: map[string]interface{}{"name": string(make([]byte, 1000))},
        want: http.StatusBadRequest,
    },
    {
        name: "Negative value",
        input: map[string]interface{}{"count": -1},
        want: http.StatusBadRequest,
    },
    {
        name: "SQL injection",
        input: map[string]interface{}{"name": "'; DROP TABLE users--"},
        want: http.StatusBadRequest,
    },
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        resp, err := client.POST("/endpoint", tt.input)
        require.NoError(t, err)
        assert.Equal(t, tt.want, resp.StatusCode)
    })
}
```

## Common Commands

```bash
# Run all tests
make test-integration

# Run specific handler tests only
go test -v ./test/integration/ -run TestProjectHandler

# Run permission tests only
go test -v ./test/integration/ -run Permission

# Run K8s-related tests only
make test-integration-k8s

# Clean test environment
make test-clean

# View test coverage
go test -v -coverprofile=coverage.out ./test/integration/...
go tool cover -html=coverage.out
```

## Debugging Tips

### 1. View Detailed Logs

```bash
go test -v ./test/integration/ -run TestName 2>&1 | tee test.log
```

### 2. Preserve Test Data

Comment out cleanup code in tests:

```go
// defer cleanupTestEnvironment()  // Comment out
```

### 3. Check K8s Resources

```bash
# View test namespaces
kubectl get ns | grep test-integration

# View test-created resources
kubectl get all -n test-integration-xxx

# View PVCs
kubectl get pvc --all-namespaces | grep test
```

### 4. Check Database

```bash
# Connect to test database
psql platform_test

# View tables
\dt

# View test data
SELECT * FROM users WHERE username LIKE 'test-%';
```

## Troubleshooting Checklist

- [ ] PostgreSQL is running normally
- [ ] Test database is created
- [ ] K8s cluster is accessible
- [ ] kubeconfig is configured correctly
- [ ] Environment variables are set correctly
- [ ] Port 8081 is not occupied
- [ ] Sufficient disk space
- [ ] Have permission to create K8s resources

## Next Steps

- Read full documentation: [README.md](README.md)
- View test implementation: `*_test.go`
- Add custom tests
- Integrate into CI/CD

## Test Report Example

```
=== RUN   TestProjectHandler_Integration
=== RUN   TestProjectHandler_Integration/GetProjects_-_Success_as_Admin
--- PASS: TestProjectHandler_Integration/GetProjects_-_Success_as_Admin (0.15s)
=== RUN   TestProjectHandler_Integration/CreateProject_-_Success_as_Admin
--- PASS: TestProjectHandler_Integration/CreateProject_-_Success_as_Admin (0.23s)
=== RUN   TestProjectHandler_Integration/CreateProject_-_Forbidden_for_Regular_User
--- PASS: TestProjectHandler_Integration/CreateProject_-_Forbidden_for_Regular_User (0.12s)
...
--- PASS: TestProjectHandler_Integration (2.45s)

=== RUN   TestK8sHandler_PVC_Integration
=== RUN   TestK8sHandler_PVC_Integration/CreatePVC_-_Success_with_K8s_Verification
--- PASS: TestK8sHandler_PVC_Integration/CreatePVC_-_Success_with_K8s_Verification (3.21s)
...
--- PASS: TestK8sHandler_PVC_Integration (15.67s)

PASS
ok      github.com/linskybing/platform-go/test/integration    45.234s
```

Success! All tests passed.
