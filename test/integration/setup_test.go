//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/linskybing/platform-go/internal/api/middleware"
	"github.com/linskybing/platform-go/internal/api/routes"
	"github.com/linskybing/platform-go/internal/config"
	"github.com/linskybing/platform-go/internal/config/db"
	"github.com/linskybing/platform-go/internal/domain/audit"
	"github.com/linskybing/platform-go/internal/domain/configfile"
	"github.com/linskybing/platform-go/internal/domain/form"
	"github.com/linskybing/platform-go/internal/domain/gpu"
	"github.com/linskybing/platform-go/internal/domain/group"
	"github.com/linskybing/platform-go/internal/domain/image"
	"github.com/linskybing/platform-go/internal/domain/job"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/domain/resource"
	"github.com/linskybing/platform-go/internal/domain/user"
	"github.com/linskybing/platform-go/internal/migrations"
	"github.com/linskybing/platform-go/pkg/k8s"
	"github.com/linskybing/platform-go/pkg/types"
)

// TestContext holds all test dependencies
type TestContext struct {
	Router        *gin.Engine
	AdminToken    string
	UserToken     string
	ManagerToken  string
	TestUser      *user.User
	TestAdmin     *user.User
	TestManager   *user.User
	TestGroup     *group.Group
	TestProject   *project.Project
	TestNamespace string
}

var testCtx *TestContext

// TestMain sets up the test environment
func TestMain(m *testing.M) {
	// Setup
	if err := setupTestEnvironment(); err != nil {
		log.Fatalf("Failed to setup test environment: %v", err)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	cleanupTestEnvironment()
	os.Exit(code)
}

func setupTestEnvironment() error {
	// Load test configuration from .env file in test/integration directory
	envPath := ".env"
	if err := godotenv.Load(envPath); err != nil {
		log.Printf("Warning: Could not load %s file: %v", envPath, err)
	}

	// Load test configuration
	_ = os.Setenv("DB_PORT", getEnvOrDefault("TEST_DB_PORT", "5432"))
	_ = os.Setenv("DB_USER", getEnvOrDefault("TEST_DB_USER", "postgres"))
	_ = os.Setenv("DB_PASSWORD", getEnvOrDefault("TEST_DB_PASSWORD", "postgres"))
	_ = os.Setenv("DB_NAME", getEnvOrDefault("TEST_DB_NAME", "platform_test"))
	_ = os.Setenv("JWT_SECRET", "test-secret-key-for-integration-testing")
	_ = os.Setenv("SERVER_PORT", "8081")
	_ = os.Setenv("ISSUER", "test-platform")

	// Load config
	config.LoadConfig()
	middleware.Init()
	config.InitK8sConfig()

	// Initialize database
	db.Init()

	// Drop and recreate tables for clean test state
	if err := db.DB.Migrator().DropTable(
		&user.User{},
		&group.Group{},
		&group.UserGroup{},
		&project.Project{},
		&configfile.ConfigFile{},
		&resource.Resource{},
		&job.Job{},
		&job.JobLog{},
		&job.JobCheckpoint{},
		&form.Form{},
		&form.FormMessage{},
		&audit.AuditLog{},
		&gpu.GPURequest{},
		&image.ImageRequest{},
		&image.AllowedImage{},
	); err != nil {
		return fmt.Errorf("failed to drop tables: %v", err)
	}

	// Auto migrate
	if err := db.DB.AutoMigrate(
		&user.User{},
		&group.Group{},
		&group.UserGroup{},
		&project.Project{},
		&configfile.ConfigFile{},
		&resource.Resource{},
		&job.Job{},
		&job.JobLog{},
		&job.JobCheckpoint{},
		&form.Form{},
		&form.FormMessage{},
		&audit.AuditLog{},
		&gpu.GPURequest{},
		&image.ImageRequest{},
		&image.AllowedImage{},
	); err != nil {
		return fmt.Errorf("failed to migrate database: %v", err)
	}
	log.Println("AutoMigrate completed")

	// Run raw SQL migrations (if any) to ensure normalized tables and schema changes are applied
	if err := migrations.RunMigrations(); err != nil {
		return fmt.Errorf("failed to run SQL migrations: %v", err)
	}

	// Create database views after tables are created
	db.CreateViews()
	log.Println("Database views created")

	// Initialize K8s client (optional - tests will skip K8s operations if unavailable)
	// Only initialize if explicitly enabled via environment variable
	k8sAvailable := false
	if os.Getenv("ENABLE_K8S_TESTS") == "true" {
		log.Println("ENABLE_K8S_TESTS is set, initializing K8s...")
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("K8s initialization failed (will skip K8s tests): %v", r)
					k8sAvailable = false
				}
			}()
			k8s.Init()
			k8sAvailable = true
			log.Println("K8s client initialized successfully")
		}()
	} else {
		log.Println("K8s tests disabled (set ENABLE_K8S_TESTS=true to enable)")
		// Fallback to a fake client so integration tests can run without a real cluster.
		k8s.InitTestCluster()
		if k8s.Clientset != nil {
			k8sAvailable = true
		}
	}

	log.Println("Setting up test context...") // Setup test context
	testCtx = &TestContext{
		TestNamespace: "test-integration-" + time.Now().Format("20060102-150405"),
	}

	// Create K8s namespace if K8s is available
	if k8sAvailable {
		if err := createTestNamespace(testCtx.TestNamespace); err != nil {
			log.Printf("Warning: failed to create test namespace: %v (K8s tests will be skipped)", err)
			k8sAvailable = false
		} else {
			log.Printf("Created test namespace: %s", testCtx.TestNamespace)
		}
	}

	// Initialize router
	log.Println("Setting Gin test mode...")
	gin.SetMode(gin.TestMode)
	log.Println("Creating new Gin router...")
	router := gin.New()
	log.Println("Adding CORS middleware...")
	router.Use(middleware.CORSMiddleware())
	log.Println("Adding logging middleware...")
	router.Use(middleware.LoggingMiddleware())
	log.Println("Router middlewares configured, registering routes...")
	routes.RegisterRoutes(router)
	testCtx.Router = router
	log.Println("Routes registered")

	// Create test data
	log.Println("Creating test data...")
	if err := createTestData(); err != nil {
		return fmt.Errorf("failed to create test data: %v", err)
	}
	log.Println("Test data created successfully")

	return nil
}

func createTestNamespace(namespace string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"test": "integration",
			},
		},
	}

	_, err := k8s.Clientset.CoreV1().Namespaces().Create(
		context.Background(),
		ns,
		metav1.CreateOptions{},
	)
	return err
}

func createTestData() error {
	// Create super admin group
	superGroup := &group.Group{
		GroupName: config.ReservedGroupName,
	}
	if err := db.DB.Create(superGroup).Error; err != nil {
		return fmt.Errorf("failed to create super group: %v", err)
	}
	testCtx.TestGroup = superGroup

	// Create test admin user
	adminEmail := "admin@test.com"
	hashedAdminPass, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	adminUser := &user.User{
		Username: "test-admin",
		Email:    &adminEmail,
		Password: string(hashedAdminPass),
	}
	if err := db.DB.Create(adminUser).Error; err != nil {
		return fmt.Errorf("failed to create admin user: %v", err)
	}
	testCtx.TestAdmin = adminUser

	// Add admin to super group
	adminUserGroup := &group.UserGroup{
		UID:  adminUser.UID,
		GID:  superGroup.GID,
		Role: "admin",
	}
	if err := db.DB.Create(adminUserGroup).Error; err != nil {
		return fmt.Errorf("failed to add admin to super group: %v", err)
	}

	// Create test regular user
	userEmail := "user@test.com"
	hashedUserPass, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	regularUser := &user.User{
		Username: "test-user",
		Email:    &userEmail,
		Password: string(hashedUserPass),
	}
	if err := db.DB.Create(regularUser).Error; err != nil {
		return fmt.Errorf("failed to create regular user: %v", err)
	}
	testCtx.TestUser = regularUser

	// Create test manager user
	managerEmail := "manager@test.com"
	hashedManagerPass, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	managerUser := &user.User{
		Username: "test-manager",
		Email:    &managerEmail,
		Password: string(hashedManagerPass),
	}
	if err := db.DB.Create(managerUser).Error; err != nil {
		return fmt.Errorf("failed to create manager user: %v", err)
	}
	testCtx.TestManager = managerUser

	// Create test project
	testProject := &project.Project{
		ProjectName: "test-project",
		GID:         superGroup.GID,
		GPUQuota:    10,       // 10 GPU units for testing
		GPUAccess:   "shared", // Shared GPU access via MPS
		MPSMemory:   1024,     // 1GB MPS memory (optional)
	}
	if err := db.DB.Create(testProject).Error; err != nil {
		return fmt.Errorf("failed to create test project: %v", err)
	}
	testCtx.TestProject = testProject

	// Add whitelisted images for testing
	testImages := []struct {
		name string
		tag  string
	}{
		{"busybox", "latest"},
		{"alpine", "latest"},
		{"nginx", "latest"},
	}

	for _, img := range testImages {
		allowedImg := &image.AllowedImage{
			Name:      img.name,
			Tag:       img.tag,
			ProjectID: &testProject.PID,
			IsGlobal:  false,
			CreatedBy: adminUser.UID,
		}
		_ = db.DB.Create(allowedImg) // Ignore errors if image already exists
	}

	// Add users to project group
	for _, u := range []*user.User{regularUser, managerUser} {
		role := "user"
		if u.UID == managerUser.UID {
			role = "manager"
		}

		userGroup := &group.UserGroup{
			UID:  u.UID,
			GID:  superGroup.GID,
			Role: role,
		}
		if err := db.DB.Create(userGroup).Error; err != nil {
			return fmt.Errorf("failed to add user to group: %v", err)
		}
	}

	// Verify admin user was added to super group
	var verifyUserGroup group.UserGroup
	if err := db.DB.Where("u_id = ? AND g_id = ?", adminUser.UID, superGroup.GID).First(&verifyUserGroup).Error; err != nil {
		return fmt.Errorf("admin user not found in super group: %v", err)
	}
	log.Printf("✓ Admin user (UID=%d) successfully added to super group (GID=%d) with role=%s",
		adminUser.UID, superGroup.GID, verifyUserGroup.Role)

	// Generate JWT tokens
	testCtx.AdminToken = generateToken(adminUser.UID, adminUser.Username)
	testCtx.UserToken = generateToken(regularUser.UID, regularUser.Username)
	testCtx.ManagerToken = generateToken(managerUser.UID, managerUser.Username)

	log.Printf("✓ Test data created: Admin(UID=%d), Manager(UID=%d), User(UID=%d)",
		adminUser.UID, managerUser.UID, regularUser.UID)

	return nil
}

func generateToken(uid uint, username string) string {
	claims := &types.Claims{
		UserID:   uid,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    config.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.JwtSecret))
	if err != nil {
		log.Fatalf("Failed to generate token: %v", err)
	}

	return tokenString
}

func cleanupTestEnvironment() {
	if testCtx == nil {
		return
	}

	// Delete test namespace only if K8s client is available
	if testCtx.TestNamespace != "" && k8s.Clientset != nil {
		_ = k8s.Clientset.CoreV1().Namespaces().Delete(
			context.Background(),
			testCtx.TestNamespace,
			metav1.DeleteOptions{},
		)
	}

	// Cleanup database
	if db.DB != nil {
		_ = db.DB.Migrator().DropTable(
			&user.User{},
			&group.Group{},
			&group.UserGroup{},
			&project.Project{},
			&configfile.ConfigFile{},
			&resource.Resource{},
			&job.Job{},
			&job.JobLog{},
			&job.JobCheckpoint{},
			&form.Form{},
			&form.FormMessage{},
			&audit.AuditLog{},
			&gpu.GPURequest{},
			&image.ImageRequest{},
			&image.AllowedImage{},
		)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetTestContext returns the global test context
func GetTestContext() *TestContext {
	return testCtx
}

// CleanupK8sResources removes all resources created during a test
func CleanupK8sResources(namespace string) error {
	ctx := context.Background()

	// Delete all pods
	_ = k8s.Clientset.CoreV1().Pods(namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{},
	)

	// Delete all PVCs
	_ = k8s.Clientset.CoreV1().PersistentVolumeClaims(namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{},
	)

	// Delete all services
	services, _ := k8s.Clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	for _, svc := range services.Items {
		_ = k8s.Clientset.CoreV1().Services(namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{})
	}

	// Delete all deployments
	_ = k8s.Clientset.AppsV1().Deployments(namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{},
	)

	// Wait a bit for resources to be deleted
	time.Sleep(2 * time.Second)

	return nil
}
