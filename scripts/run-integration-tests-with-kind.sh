#!/bin/bash
set -e

echo "=========================================="
echo "Integration Test with Kind Kubernetes"
echo "=========================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="platform-test-cluster"
POSTGRES_CONTAINER="postgres-integration-test"
TEST_NAMESPACE="test-integration"

# Function to cleanup
cleanup() {
    echo -e "\n${YELLOW}Cleaning up resources...${NC}"
    
    # Stop PostgreSQL
    if docker ps -a --format '{{.Names}}' | grep -q "^${POSTGRES_CONTAINER}$"; then
        echo "Stopping PostgreSQL container..."
        docker stop ${POSTGRES_CONTAINER} >/dev/null 2>&1 || true
        docker rm ${POSTGRES_CONTAINER} >/dev/null 2>&1 || true
    fi
    
    # Delete Kind cluster
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        echo "Deleting Kind cluster..."
        kind delete cluster --name ${CLUSTER_NAME}
    fi
    
    echo -e "${GREEN}Cleanup completed${NC}"
}

# Trap to ensure cleanup on exit
trap cleanup EXIT

echo ""
echo "[1/6] Starting PostgreSQL..."
docker run --name ${POSTGRES_CONTAINER} \
  -e POSTGRES_USER=test \
  -e POSTGRES_PASSWORD=test \
  -e POSTGRES_DB=platform_test \
  -p 5432:5432 \
  --health-cmd="pg_isready -U test" \
  --health-interval=10s \
  --health-timeout=5s \
  --health-retries=5 \
  -d postgres:15

echo ""
echo "[2/6] Waiting for PostgreSQL to be ready..."
for i in {1..30}; do
  if docker exec ${POSTGRES_CONTAINER} pg_isready -U test > /dev/null 2>&1; then
    echo -e "${GREEN}✓ PostgreSQL is ready${NC}"
    break
  fi
  echo "  Attempt $i/30..."
  sleep 2
done

echo ""
echo "[3/6] Creating Kind Kubernetes cluster..."
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    echo "Kind cluster already exists, deleting..."
    kind delete cluster --name ${CLUSTER_NAME}
fi

cat <<EOF | kind create cluster --name ${CLUSTER_NAME} --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 30000
    hostPort: 30000
    protocol: TCP
EOF

echo ""
echo "[4/6] Verifying Kubernetes cluster..."
kubectl cluster-info --context kind-${CLUSTER_NAME}
kubectl get nodes

echo ""
echo "[5/6] Creating test namespace..."
kubectl create namespace ${TEST_NAMESPACE} || true
kubectl get namespaces

echo ""
echo "[6/6] Running integration tests..."
echo "=========================================="

export TEST_DB_HOST=localhost
export TEST_DB_PORT=5432
export TEST_DB_USER=test
export TEST_DB_PASSWORD=test
export TEST_DB_NAME=platform_test
export KUBECONFIG="${HOME}/.kube/config"

cd "$(dirname "$0")/.."

go test -v -count=1 -timeout 15m ./test/integration/... 2>&1 | tee /tmp/integration-test-kind.log

TEST_EXIT_CODE=${PIPESTATUS[0]}

echo ""
echo "=========================================="
if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}✓ Integration tests PASSED${NC}"
    echo "=========================================="
    exit 0
else
    echo -e "${RED}✗ Integration tests FAILED${NC}"
    echo "=========================================="
    echo "Full log saved to: /tmp/integration-test-kind.log"
    exit 1
fi
