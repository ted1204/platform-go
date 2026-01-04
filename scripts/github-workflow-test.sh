#!/bin/bash

set -e

echo "=========================================="
echo "  Simulating GitHub Actions Workflow"
echo "=========================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up resources..."
    docker rm -f postgres-test-ci 2>/dev/null || true
    kind delete cluster --name test-cluster 2>/dev/null || true
}

# Set trap for cleanup on exit
trap cleanup EXIT

echo "[1/7] Starting PostgreSQL (simulating GitHub Actions service)..."
docker run -d \
    --name postgres-test-ci \
    -e POSTGRES_USER=test \
    -e POSTGRES_PASSWORD=test \
    -e POSTGRES_DB=platform_test \
    -p 5433:5432 \
    --health-cmd="pg_isready -U test" \
    --health-interval=10s \
    --health-timeout=5s \
    --health-retries=5 \
    postgres:15

echo "Waiting for PostgreSQL to be ready..."
for i in {1..30}; do
    if docker exec postgres-test-ci pg_isready -U test > /dev/null 2>&1; then
        echo -e "${GREEN}PostgreSQL is ready!${NC}"
        break
    fi
    echo "Attempt $i/30..."
    sleep 2
done

echo ""
echo "[2/7] Setting up Kind Kubernetes cluster..."
if kind get clusters | grep -q test-cluster; then
    echo "Deleting existing test cluster..."
    kind delete cluster --name test-cluster
fi

kind create cluster --name test-cluster --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: ClusterConfiguration
    apiServer:
      extraArgs:
        "enable-admission-plugins": "NodeRestriction"
EOF

echo ""
echo "[3/7] Verifying Kubernetes cluster..."
kubectl cluster-info
kubectl get nodes

echo ""
echo "[4/7] Creating test namespace..."
kubectl create namespace integration-test || true

echo ""
echo "[5/7] Running Go mod download..."
go mod download

echo ""
echo "[6/7] Running integration tests with K8s enabled..."
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5433
export TEST_DB_USER=test
export TEST_DB_PASSWORD=test
export TEST_DB_NAME=platform_test
export ENABLE_K8S_TESTS=true

# Run tests
if go test -v -count=1 -timeout 15m ./test/integration/... 2>&1 | tee ci_test_output.log; then
    echo -e "${GREEN}"
    echo "=========================================="
    echo "  ✓ All tests passed!"
    echo "=========================================="
    echo -e "${NC}"
    exit 0
else
    echo -e "${RED}"
    echo "=========================================="
    echo "  ✗ Tests failed - Check logs above"
    echo "=========================================="
    echo -e "${NC}"
    
    # Show summary
    echo ""
    echo "Test Summary:"
    grep -E "^(PASS|FAIL|SKIP):" ci_test_output.log || true
    
    exit 1
fi
