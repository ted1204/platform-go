# Integration Testing Documentation

## Overview

This document explains how to run and maintain integration tests for the platform-go project. Integration tests cover all API handlers, including boundary condition tests, permission management validation, and Kubernetes resource operation verification.

## Test Architecture

### Test Components

1. **Test Framework** (`setup_test.go`)
   - Initialize test environment (database, K8s, MinIO)
   - Create test data (users, groups, projects)
   - Generate test JWT tokens
   - Test cleanup and resource recycling

2. **HTTP Client** (`http_client.go`)
   - Wrap HTTP requests/responses
   - Automatically handle JWT authentication
   - JSON serialization/deserialization

3. **K8s Validator** (`k8s_validator.go`)
   - Validate K8s resource status
   - Wait for resources to be ready
   - Create/delete test resources

4. **Test Helper Tools** (`helpers.go`)
   - Test data generator
   - Resource cleanup tools
   - Performance timer

### Handler Test Coverage

- Project Handler - Project CRUD and permissions
- ConfigFile Handler - Configuration files and instance management
- User Handler - User authentication and authorization
- Group Handler - Group management
- UserGroup Handler - User-group relationships and roles
- K8s Handler - PVC, user storage, project storage, Jobs
- GPU Request Handler - GPU request workflow
- Form Handler - Form submission and approval
- Audit Handler - Audit logs

## Environment Setup

### Prerequisites

1. **Kubernetes Cluster**
   ```bash
   # Ensure K8s cluster is accessible
   kubectl cluster-info
   kubectl get nodes
   ```

2. **PostgreSQL Database**
   ```bash
   # Create test database
   createdb platform_test
   ```

3. **Environment Variables**
   Create `.env.test` file:
   ```env
   TEST_DB_HOST=localhost
   TEST_DB_PORT=5432
   TEST_DB_USER=postgres
   TEST_DB_PASSWORD=postgres
   TEST_DB_NAME=platform_test
   JWT_SECRET=test-secret-key-for-integration-testing
   SERVER_PORT=8081
   ISSUER=test-platform
   
   # MinIO configuration (optional)
   MINIO_ENDPOINT=localhost:9000
   MINIO_ACCESS_KEY=minioadmin
   MINIO_SECRET_KEY=minioadmin
   MINIO_USE_SSL=false
   ```

### Install Dependencies

```bash
go mod download
go install github.com/golang/mock/mockgen@latest
```

## Running Tests

### Run All Integration Tests

```bash
# Basic run
go test -v ./test/integration/...

# Run with timeout
go test -v -timeout 30m ./test/integration/...

# Parallel run (use with caution, may cause K8s resource conflicts)
go test -v -parallel 2 ./test/integration/...
```

### Run Specific Tests

```bash
# Run only Project Handler tests
go test -v ./test/integration/ -run TestProjectHandler

# Run specific test case
go test -v ./test/integration/ -run TestProjectHandler_Integration/CreateProject

# Run permission tests
go test -v ./test/integration/ -run Permission
```

### Using Makefile

Add test commands to `Makefile`:

```bash
make test-integration        # Run all integration tests
make test-integration-quick  # Quick tests (skip slow tests)
make test-integration-k8s    # Run only K8s-related tests
make test-clean              # Clean test environment
```

## Test Case Description

### 1. Project Handler Tests

**Test Scenarios:**
- Create/Read/Update/Delete projects
- Permission validation (Admin, Manager, User)
- Input validation (empty name, invalid GID)
- Project PVC creation and K8s validation

**Key Tests:**
- `CreateProject - Success as Admin`: Verify admin can create projects
- `CreateProject - Forbidden for Regular User`: Verify regular users cannot create projects
- `CreateProjectPVC - Success with K8s Verification`: Verify PVC is created successfully in K8s

### 2. ConfigFile Handler Tests

**Test Scenarios:**
- Configuration file CRUD
- Instance creation/destruction
- Resource limit validation
- K8s Deployment validation

**Key Tests:**
- `CreateInstance - Success with K8s Verification`: Verify deployment is created in K8s
- `ResourceLimits`: Verify CPU/memory limits are set correctly

### 3. User Handler Tests

**Test Scenarios:**
- User registration/login/logout
- Password security validation
- SQL injection protection
- XSS protection
- Self-update vs updating others

**Key Tests:**
- `Register - Weak Password`: Verify weak passwords are rejected
- `UpdateUser - Forbidden Other User`: Verify cannot modify other users' information
- `InputValidation`: Verify SQL injection and XSS protection

### 4. K8s Handler Tests

**Test Scenarios:**
- PVC creation/expansion/deletion
- User storage initialization
- Project storage management
- K8s resource status validation

**Key Tests:**
- `CreatePVC - Success with K8s Verification`: Verify PVC is created in K8s
- `ExpandPVC - Cannot Shrink`: Verify PVC cannot be shrunk
- `DeletePVC - Success with K8s Verification`: Verify PVC is deleted from K8s

### 5. Permission Boundary Tests

Each handler has permission boundary tests:
- Admin level operations
- Manager level operations
- User level operations
- Unauthorized access tests

## Test Data Description

The test framework automatically creates the following test data:

- **Super Group**: `super` (GID: dynamic)
- **Admin User**: `test-admin`
- **Regular User**: `test-user`
- **Manager User**: `test-manager`
- **Test Project**: `test-project`
- **Test Namespace**: `test-integration-{timestamp}`

All data is automatically cleaned up after tests complete.

## Troubleshooting

### Common Issues

**1. Database Connection Failure**
```bash
# Check if PostgreSQL is running
pg_isready -h localhost -p 5432

# Check if database exists
psql -l | grep platform_test
```

**2. K8s Connection Failure**
```bash
# Check kubeconfig
kubectl config view
kubectl cluster-info

# Check permissions
kubectl auth can-i create pods --all-namespaces
```

**3. Test Timeout**
```bash
# Increase timeout
go test -v -timeout 60m ./test/integration/...

# View slow tests
go test -v -timeout 30m ./test/integration/... | grep -E "PASS|FAIL"
```

**4. K8s Resource Residue**
```bash
# Manually clean test namespaces
kubectl get ns | grep test-integration | awk '{print $1}' | xargs kubectl delete ns

# Clean test PVCs
kubectl get pvc --all-namespaces | grep test
```

**5. Port Conflict**
```bash
# Check port usage
lsof -i :8081

# Modify SERVER_PORT environment variable
```

## Continuous Integration

### GitHub Actions Configuration

Create `.github/workflows/integration-test.yml`:

```yaml
name: Integration Tests

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main, develop ]

jobs:
  integration-test:
    runs-on: ubuntu-latest
    
    services:
      postgres:
        image: postgres:14
        env:
          POSTGRES_DB: platform_test
          POSTGRES_USER: postgres
          POSTGRES_PASSWORD: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5432:5432
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.24'
    
    - name: Set up K8s (Kind)
      uses: helm/kind-action@v1.5.0
      with:
        cluster_name: test-cluster
    
    - name: Run Integration Tests
      env:
        TEST_DB_HOST: localhost
        TEST_DB_PORT: 5432
        TEST_DB_USER: postgres
        TEST_DB_PASSWORD: postgres
        TEST_DB_NAME: platform_test
      run: |
        go test -v -timeout 30m ./test/integration/...
```

## Performance Benchmark

Expected test execution time:

- Project Handler: ~10s
- ConfigFile Handler: ~15s
- User Handler: ~8s
- Group Handler: ~12s
- K8s Handler: ~30s (including resource creation wait)
- Total: ~2-3 minutes

## Best Practices

1. **Isolate Test Data**
   - Use separate test database
   - Use dedicated K8s namespaces for testing
   - Avoid affecting production data

2. **Clean Up Resources**
   - Clean up created resources after each test
   - Use `defer` to ensure cleanup execution
   - Periodically check and clean residual resources

3. **Idempotency**
   - Tests should be repeatable
   - Do not depend on previous test state
   - Use random data to avoid conflicts

4. **Timeout Settings**
   - Set reasonable timeouts for K8s operations
   - Avoid infinite waiting in tests
   - Use context.WithTimeout

5. **Error Handling**
   - Log detailed failure reasons
   - Preserve state information on failure
   - Provide useful error messages

## Extending Tests

### Adding New Tests

1. Create new test file: `{handler}_handler_test.go`
2. Use `TestContext` to get test environment
3. Use `HTTPClient` to send requests
4. Use `K8sValidator` to verify K8s resources
5. Use `assert`/`require` for assertions

Example:

```go
func TestNewHandler_Integration(t *testing.T) {
    ctx := GetTestContext()
    k8sValidator := NewK8sValidator()
    
    t.Run("TestCase - Description", func(t *testing.T) {
        client := NewHTTPClient(ctx.Router, ctx.AdminToken)
        
        resp, err := client.POST("/endpoint", requestBody)
        require.NoError(t, err)
        assert.Equal(t, http.StatusOK, resp.StatusCode)
        
        // K8s validation
        exists, err := k8sValidator.PodExists(namespace, podName)
        require.NoError(t, err)
        assert.True(t, exists)
    })
}
```

## Maintenance Recommendations

1. **Regular Execution**
   - Run before each code commit
   - Automatically run in CI/CD pipeline
   - Full regression testing weekly

2. **Update Tests**
   - Update corresponding tests when API changes
   - Add tests when adding new features
   - Add regression tests when fixing bugs

3. **Monitor Performance**
   - Track test execution time
   - Identify slow tests
   - Optimize test efficiency

4. **Documentation Maintenance**
   - Update test scenario documentation
   - Document known issues
   - Maintain troubleshooting guide

## Contact

For questions or suggestions, please contact the development team or submit an Issue.
