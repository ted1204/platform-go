package unit_tests

import (
	"testing"

	"github.com/linskybing/platform-go/config"
	"github.com/linskybing/platform-go/minio"
)

func TestMain(m *testing.M) {
	config.LoadConfig()
	config.InitK8sConfig()
	minio.InitMinio()
	m.Run()
}
