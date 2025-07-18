package utils

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/linskybing/platform-go/minio"
	minioSDK "github.com/minio/minio-go/v7"
)

// UploadObject uploads content as an object to MinIO with specified content-type.
// objectName: the target object name (e.g. "config.yaml" or "data.json")
// contentType: MIME type like "application/x-yaml", "application/json", "text/plain" etc.
// contentReader: io.Reader with the data to upload
func UploadObject(ctx context.Context, objectName string, contentType string, contentReader io.Reader, contentSize int64) error {
	if strings.TrimSpace(objectName) == "" {
		return fmt.Errorf("object name cannot be empty")
	}

	_, err := minio.Client.PutObject(ctx, minio.BucketName, objectName, contentReader, contentSize, minioSDK.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// UploadString uploads a string as an object to MinIO with given content-type.
func UploadString(ctx context.Context, objectName string, contentType string, content string) error {
	return UploadObject(ctx, objectName, contentType, strings.NewReader(content), int64(len(content)))
}

// DownloadObject downloads object content from MinIO and returns as []byte.
func DownloadObject(ctx context.Context, objectName string) ([]byte, error) {
	obj, err := minio.Client.GetObject(ctx, minio.BucketName, objectName, minioSDK.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()

	return io.ReadAll(obj)
}

// DownloadString downloads object content and returns it as string.
func DownloadString(ctx context.Context, objectName string) (string, error) {
	data, err := DownloadObject(ctx, objectName)
	return string(data), err
}

// DeleteObject deletes the specified object from MinIO bucket.
func DeleteObject(ctx context.Context, objectName string) error {
	return minio.Client.RemoveObject(ctx, minio.BucketName, objectName, minioSDK.RemoveObjectOptions{})
}
