#!/bin/bash
set -e

CONFIG_FILE="/etc/containerd/config.toml"
IP="192.168.109.1"
PORT="30002"

echo "=== 核彈級修復：使用正則表達式清理設定檔 ==="

# 1. 重建預設設定檔 (確保檔案存在且完整)
echo "[STEP 1] 重建預設設定..."
sudo mkdir -p /etc/containerd
sudo containerd config default | sudo tee "$CONFIG_FILE" > /dev/null

# 2. 強力替換 (Regex Replace)
# 不管原本是 config_path = "" 還是 config_path = "..."
# 只要開頭是 config_path，整行直接換掉！
echo "[STEP 2] 強制修正 Registry 路徑..."
sudo sed -i 's|config_path = .*|config_path = "/etc/containerd/certs.d"|g' "$CONFIG_FILE"

# 3. 啟用 Systemd Cgroup
echo "[STEP 3] 啟用 SystemdCgroup..."
sudo sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' "$CONFIG_FILE"

# 4. 恢復 NVIDIA 設定
echo "[STEP 4] 恢復 NVIDIA Runtime..."
sudo nvidia-ctk runtime configure --runtime=containerd --set-as-default

# 5. 再次驗證 (這次絕對不能有兩行！)
echo "---------------------------------------------"
echo "[驗證] 目前的 config_path (應該只有一行):"
grep "config_path" "$CONFIG_FILE" | grep "certs.d"
echo "---------------------------------------------"

echo "[STEP 5] 重啟 Containerd..."
sudo systemctl restart containerd

echo "=== 最終測試 ==="
echo "嘗試拉取映像檔..."
sudo crictl pull $IP:$PORT/platform/postgres-with-pg_cron:latest