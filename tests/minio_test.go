package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/linskybing/platform-go/config"
	"github.com/linskybing/platform-go/minio"
	"github.com/linskybing/platform-go/utils"
)

func TestUploadDownloadYAML(t *testing.T) {
	config.LoadConfig()
	minio.InitMinio()

	ctx := context.Background()
	start := time.Now()

	testObject := "test/test-config.yaml"
	testContent := "key: value\nfoo: bar\n"

	if err := utils.UploadObject(ctx, testObject, "application/x-yaml", strings.NewReader(testContent), int64(len(testContent))); err != nil {
		t.Fatalf("UploadYAML failed: %v", err)
	}

	content, err := utils.DownloadString(ctx, testObject)
	if err != nil {
		t.Fatalf("DownloadYAML failed: %v", err)
	}

	if content != testContent {
		t.Fatalf("Downloaded content mismatch.\nGot:\n%s\nWant:\n%s", content, testContent)
	}

	elapsed := time.Since(start)
	t.Logf("TestUploadDownloadYAML took %s", elapsed)
}

func TestMain(m *testing.M) {
	config.LoadConfig()
	minio.InitMinio()
	m.Run()
}
