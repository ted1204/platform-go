#!/bin/bash
set -e

# ==============================================================================
# SCRIPT: Build & Push to Private Harbor
# SCOPE:  Builds Go API & Postgres images and pushes to internal registry
# ==============================================================================

# --- Configuration ---
# Registry details from your previous setup
HARBOR_HOST="192.168.109.1:30002"
PROJECT_NAME="platform"   # Ensure this project exists in Harbor UI
TAG="latest"

# Image Names
IMG_API="platform-go-api"
IMG_DB="postgres-with-pg_cron"

# Full Image Paths
TARGET_API="${HARBOR_HOST}/${PROJECT_NAME}/${IMG_API}:${TAG}"
TARGET_DB="${HARBOR_HOST}/${PROJECT_NAME}/${IMG_DB}:${TAG}"

echo "=== Starting Build Process for Registry: $HARBOR_HOST ==="

# ------------------------------------------------------------------------------
# 1. Build & Push Go API
# ------------------------------------------------------------------------------
echo "[STEP 1] Building Go API..."
# We build directly with the target tag to save an extra 'docker tag' step
docker build -t "$TARGET_API" .

echo "[STEP 2] Pushing Go API to Harbor..."
docker push "$TARGET_API"

# ------------------------------------------------------------------------------
# 2. Build & Push Postgres
# ------------------------------------------------------------------------------
echo "[STEP 3] Building Postgres image..."
docker build -t "$TARGET_DB" -f infra/db/postgres/Dockerfile infra/db/postgres

echo "[STEP 4] Pushing Postgres to Harbor..."
docker push "$TARGET_DB"

echo "========================================================"
echo "Build & Push Complete."
echo "   - API: $TARGET_API"
echo "   - DB:  $TARGET_DB"
echo "========================================================"