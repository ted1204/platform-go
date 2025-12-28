#!/bin/bash

# Exit on error
set -e

echo "Starting Development Environment..."

# kubectl create secret docker-registry harbor-cred \
#   --docker-server=192.168.109.1:30002 \
#   --docker-username=admin \
#   --docker-password=Harbor12345
# 1. Apply Kubernetes Manifests
echo "Applying K8s manifests..."
kubectl apply -f k8s/ca.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/postgres.yaml
kubectl apply -f k8s/go-api.yaml

# 2. Wait for Pods to be Ready
echo "Waiting for go-api deployment to be ready..."
kubectl rollout status deployment/go-api --timeout=120s

# 3. Get the Pod Name
# We add a sleep to ensure the status is fully synced before grabbing the name
sleep 2
POD_NAME=$(kubectl get pods -l app=go-api -o jsonpath="{.items[0].metadata.name}")

if [ -z "$POD_NAME" ]; then
    echo "Error: Could not find go-api pod."
    exit 1
fi

echo "Found pod: $POD_NAME"

# 4. Run the Application
echo "Starting Go application inside the pod..."
echo "Source code is mounted at /go/web-go"
echo "Press Ctrl+C to stop the application (the pod will remain running)"
echo "----------------------------------------------------------------"

# Execute the Go run command directly inside the container
# We use 'bash -c' to run the command chain
kubectl exec -it $POD_NAME -- bash -c "cd /go/web-go && export GOTOOLCHAIN=local && export GOSUMDB=off && go run src/main.go"