docker run -p 9000:9000 -p 9001:9001 --name minio `
  -e "MINIO_ROOT_USER=minioadmin" `
  -e "MINIO_ROOT_PASSWORD=minioadmin" `
  -v C:\Users\linsk\Desktop\data:/data `
  minio/minio server /data --console-address ":9001"

  @REM docker run -p 9000:9000 -p 9001:9001 --name minio \
  @REM -e "MINIO_ROOT_USER=minioadmin" \
  @REM -e "MINIO_ROOT_PASSWORD=minioadmin" \
  @REM -v /Users/linbinghong/Desktop/data:/data \
  @REM minio/minio server /data --console-address ":9001"
