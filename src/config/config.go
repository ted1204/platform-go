package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	JwtSecret               string
	DbHost                  string
	DbPort                  string
	DbUser                  string
	DbPassword              string
	DbName                  string
	ServerPort              string
	Issuer                  string
	GroupAdminRoles         = []string{"admin"}
	GroupUpdateRoles        = []string{"admin", "manager"}
	MinioEndpoint           string
	MinioAccessKey          string
	MinioSecretKey          string
	MinioUseSSL             bool
	MinioBucket             string
	Scheme                  = runtime.NewScheme()
	DefaultStorageName      = "project"
	DefaultStorageClassName = "longhorn"
	DefaultStorageSize      = "3Gi"
)

func LoadConfig() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	JwtSecret = getEnv("JWT_SECRET", "defaultsecret")
	DbHost = getEnv("DB_HOST", "localhost")
	DbPort = getEnv("DB_PORT", "5432")
	DbUser = getEnv("DB_USER", "postgres")
	DbPassword = getEnv("DB_PASSWORD", "password")
	DbName = getEnv("DB_NAME", "platform")
	ServerPort = getEnv("SERVER_PORT", "8080")
	Issuer = getEnv("Issuer", "platform")

	MinioEndpoint = getEnv("MINIO_ENDPOINT", "minio.tenant.svc.cluster.local:443")
	MinioAccessKey = getEnv("MINIO_ACCESS_KEY", "minio")
	MinioSecretKey = getEnv("MINIO_SECRET_KEY", "minio123")
	MinioBucket = getEnv("MINIO_BUCKET", "platform-bucket")
	MinioUseSSL, _ = strconv.ParseBool(getEnv("MINIO_USE_SSL", "true"))
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func InitK8sConfig() {
	_ = corev1.AddToScheme(Scheme)
	_ = appsv1.AddToScheme(Scheme)
}
