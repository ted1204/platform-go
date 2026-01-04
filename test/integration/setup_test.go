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
	"github.com/linskybing/platform-go/internal/domain/job"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/domain/resource"
	"github.com/linskybing/platform-go/internal/domain/user"
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
	// Load test configuration
	os.Setenv("DB_HOST", getEnvOrDefault("TEST_DB_HOST", "localhost"))
	os.Setenv("DB_PORT", getEnvOrDefault("TEST_DB_PORT", "5432"))
	os.Setenv("DB_USER", getEnvOrDefault("TEST_DB_USER", "postgres"))
	os.Setenv("DB_PASSWORD", getEnvOrDefault("TEST_DB_PASSWORD", "postgres"))
	os.Setenv("DB_NAME", getEnvOrDefault("TEST_DB_NAME", "platform_test"))
	os.Setenv("JWT_SECRET", "test-secret-key-for-integration-testing")
	os.Setenv("SERVER_PORT", "8081")
	os.Setenv("ISSUER", "test-platform")

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
		&form.Form{},
		&audit.AuditLog{},
		&gpu.GPURequest{},
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
		&form.Form{},
		&audit.AuditLog{},
		&gpu.GPURequest{},
	); err != nil {
		return fmt.Errorf("failed to migrate database: %v", err)
	}
	log.Println("AutoMigrate completed")

	// Create database views after tables are created
	if err := createDatabaseViews(); err != nil {
		return fmt.Errorf("failed to create database views: %v", err)
	}
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
	adminUser := &user.User{
		Username: "test-admin",
		Email:    &adminEmail,
		Password: "password123",
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
	regularUser := &user.User{
		Username: "test-user",
		Email:    &userEmail,
		Password: "password123",
	}
	if err := db.DB.Create(regularUser).Error; err != nil {
		return fmt.Errorf("failed to create regular user: %v", err)
	}
	testCtx.TestUser = regularUser

	// Create test manager user
	managerEmail := "manager@test.com"
	managerUser := &user.User{
		Username: "test-manager",
		Email:    &managerEmail,
		Password: "password123",
	}
	if err := db.DB.Create(managerUser).Error; err != nil {
		return fmt.Errorf("failed to create manager user: %v", err)
	}
	testCtx.TestManager = managerUser

	// Create test project
	testProject := &project.Project{
		ProjectName: "test-project",
		GID:         superGroup.GID,
	}
	if err := db.DB.Create(testProject).Error; err != nil {
		return fmt.Errorf("failed to create test project: %v", err)
	}
	testCtx.TestProject = testProject

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

	// Delete test namespace
	if testCtx.TestNamespace != "" {
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
			&form.Form{},
			&audit.AuditLog{},
			&gpu.GPURequest{},
		)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// createDatabaseViews creates all necessary database views
func createDatabaseViews() error {
	views := []string{
		`CREATE OR REPLACE VIEW user_group_views AS
		SELECT
			u.u_id,
			u.username,
			g.g_id,
			g.group_name,
			ug.role
		FROM users u
		JOIN user_group ug ON u.u_id = ug.u_id
		JOIN group_list g ON ug.g_id = g.g_id`,

		`CREATE OR REPLACE VIEW users_with_superadmin AS
		SELECT
			u.u_id,
			u.username,
			u.password,
			u.email,
			u.full_name,
			u.type,
			u.status,
			u.create_at,
			u.update_at,
			CASE WHEN ug.role = 'admin' AND ug.group_name = 'super' THEN true ELSE false END AS is_super_admin
		FROM users u
		LEFT JOIN user_group_views ug ON u.u_id = ug.u_id AND ug.group_name = 'super' AND ug.role = 'admin'`,

		`CREATE OR REPLACE VIEW project_group_views AS
		SELECT
			g.g_id,
			g.group_name,
			COUNT(DISTINCT p.p_id) AS project_count,
			COUNT(r.r_id) AS resource_count,
			MAX(g.create_at) AS group_create_at,
			MAX(g.update_at) AS group_update_at
		FROM group_list g
		LEFT JOIN project_list p ON p.g_id = g.g_id
		LEFT JOIN config_files cf ON cf.project_id = p.p_id
		LEFT JOIN resource_list r ON r.cf_id = cf.cf_id
		GROUP BY g.g_id, g.group_name`,

		`CREATE OR REPLACE VIEW project_resource_views AS
		SELECT
			p.p_id,
			p.project_name,
			r.r_id,
			r.type,
			r.name,
			cf.filename,
			r.create_at AS resource_create_at
		FROM project_list p
		JOIN config_files cf ON cf.project_id = p.p_id
		JOIN resource_list r ON r.cf_id = cf.cf_id`,

		`CREATE OR REPLACE VIEW group_resource_views AS
		SELECT
			g.g_id,
			g.group_name,
			p.p_id,
			p.project_name,
			r.r_id,
			r.type AS resource_type,
			r.name AS resource_name,
			cf.filename,
			r.create_at AS resource_create_at
		FROM group_list g
		LEFT JOIN project_list p ON p.g_id = g.g_id
		LEFT JOIN config_files cf ON cf.project_id = p.p_id
		LEFT JOIN resource_list r ON r.cf_id = cf.cf_id
		WHERE r.r_id IS NOT NULL`,

		`CREATE OR REPLACE VIEW project_user_views AS
		SELECT
			p.p_id,
			p.project_name,
			g.g_id,
			g.group_name,
			u.u_id,
			u.username,
			ug.role
		FROM project_list p
		JOIN group_list g ON p.g_id = g.g_id
		JOIN user_group ug ON ug.g_id = g.g_id
		JOIN users u ON u.u_id = ug.u_id`,
	}

	for _, view := range views {
		if err := db.DB.Exec(view).Error; err != nil {
			return fmt.Errorf("failed to create view: %v", err)
		}
	}

	return nil
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
