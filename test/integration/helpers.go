//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/linskybing/platform-go/internal/config/db"
	"github.com/linskybing/platform-go/internal/domain/group"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/domain/user"
	"github.com/linskybing/platform-go/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestDataGenerator generates test data for integration tests
type TestDataGenerator struct {
	rand *rand.Rand
}

// NewTestDataGenerator creates a new test data generator
func NewTestDataGenerator() *TestDataGenerator {
	return &TestDataGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateUser generates a random test user
func (g *TestDataGenerator) GenerateUser(prefix string) *user.User {
	email := fmt.Sprintf("%s-%d@test.com", prefix, g.rand.Intn(10000))
	return &user.User{
		Username: fmt.Sprintf("%s-user-%d", prefix, g.rand.Intn(10000)),
		Email:    &email,
		Password: "password123",
	}
}

// GenerateUsers generates multiple test users
func (g *TestDataGenerator) GenerateUsers(prefix string, count int) []*user.User {
	users := make([]*user.User, count)
	for i := 0; i < count; i++ {
		users[i] = g.GenerateUser(prefix)
	}
	return users
}

// GenerateGroup generates a random test group
func (g *TestDataGenerator) GenerateGroup(prefix string) *group.Group {
	return &group.Group{
		GroupName: fmt.Sprintf("%s-group-%d", prefix, g.rand.Intn(10000)),
	}
}

// GenerateProject generates a random test project
func (g *TestDataGenerator) GenerateProject(prefix string, gid uint) *project.Project {
	return &project.Project{
		ProjectName: fmt.Sprintf("%s-project-%d", prefix, g.rand.Intn(10000)),
		GID:         gid,
	}
}

// CreateTestUser creates a user in the database
func (g *TestDataGenerator) CreateTestUser(u *user.User) error {
	return db.DB.Create(u).Error
}

// CreateTestGroup creates a group in the database
func (g *TestDataGenerator) CreateTestGroup(grp *group.Group) error {
	return db.DB.Create(grp).Error
}

// CreateTestProject creates a project in the database
func (g *TestDataGenerator) CreateTestProject(p *project.Project) error {
	return db.DB.Create(p).Error
}

// AddUserToGroup adds a user to a group with specified role
func (g *TestDataGenerator) AddUserToGroup(uid, gid uint, role string) error {
	ug := &group.UserGroup{
		UID:  uid,
		GID:  gid,
		Role: role,
	}
	return db.DB.Create(ug).Error
}

// K8sResourceCleaner cleans up K8s resources after tests
type K8sResourceCleaner struct {
	namespaces []string
}

// NewK8sResourceCleaner creates a new K8s resource cleaner
func NewK8sResourceCleaner() *K8sResourceCleaner {
	return &K8sResourceCleaner{
		namespaces: make([]string, 0),
	}
}

// RegisterNamespace registers a namespace for cleanup
func (c *K8sResourceCleaner) RegisterNamespace(namespace string) {
	c.namespaces = append(c.namespaces, namespace)
}

// Cleanup removes all registered resources
func (c *K8sResourceCleaner) Cleanup() error {
	ctx := context.Background()

	for _, ns := range c.namespaces {
		// Delete all pods
		_ = k8s.Clientset.CoreV1().Pods(ns).DeleteCollection(
			ctx,
			metav1.DeleteOptions{},
			metav1.ListOptions{},
		)

		// Delete all PVCs
		_ = k8s.Clientset.CoreV1().PersistentVolumeClaims(ns).DeleteCollection(
			ctx,
			metav1.DeleteOptions{},
			metav1.ListOptions{},
		)

		// Delete all services
		services, _ := k8s.Clientset.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
		for _, svc := range services.Items {
			_ = k8s.Clientset.CoreV1().Services(ns).Delete(ctx, svc.Name, metav1.DeleteOptions{})
		}

		// Delete all deployments
		_ = k8s.Clientset.AppsV1().Deployments(ns).DeleteCollection(
			ctx,
			metav1.DeleteOptions{},
			metav1.ListOptions{},
		)

		// Delete all jobs
		_ = k8s.Clientset.BatchV1().Jobs(ns).DeleteCollection(
			ctx,
			metav1.DeleteOptions{},
			metav1.ListOptions{},
		)

		// Delete namespace (if it's a test namespace)
		if isTestNamespace(ns) {
			_ = k8s.Clientset.CoreV1().Namespaces().Delete(
				ctx,
				ns,
				metav1.DeleteOptions{},
			)
		}
	}

	// Wait a bit for resources to be deleted
	time.Sleep(3 * time.Second)

	return nil
}

func isTestNamespace(namespace string) bool {
	testPrefixes := []string{
		"test-",
		"integration-",
		"temp-",
	}

	for _, prefix := range testPrefixes {
		if len(namespace) >= len(prefix) && namespace[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}

// DatabaseCleaner cleans up database records after tests
type DatabaseCleaner struct {
	userIDs    []uint
	groupIDs   []uint
	projectIDs []uint
}

// NewDatabaseCleaner creates a new database cleaner
func NewDatabaseCleaner() *DatabaseCleaner {
	return &DatabaseCleaner{
		userIDs:    make([]uint, 0),
		groupIDs:   make([]uint, 0),
		projectIDs: make([]uint, 0),
	}
}

// RegisterUser registers a user for cleanup
func (c *DatabaseCleaner) RegisterUser(uid uint) {
	c.userIDs = append(c.userIDs, uid)
}

// RegisterGroup registers a group for cleanup
func (c *DatabaseCleaner) RegisterGroup(gid uint) {
	c.groupIDs = append(c.groupIDs, gid)
}

// RegisterProject registers a project for cleanup
func (c *DatabaseCleaner) RegisterProject(pid uint) {
	c.projectIDs = append(c.projectIDs, pid)
}

// Cleanup removes all registered database records
func (c *DatabaseCleaner) Cleanup() error {
	// Delete in reverse order of dependencies

	// Delete user groups
	for _, uid := range c.userIDs {
		_ = db.DB.Where("uid = ?", uid).Delete(&group.UserGroup{}).Error
	}
	for _, gid := range c.groupIDs {
		_ = db.DB.Where("gid = ?", gid).Delete(&group.UserGroup{}).Error
	}

	// Delete projects
	for _, pid := range c.projectIDs {
		_ = db.DB.Delete(&project.Project{}, pid).Error
	}

	// Delete users
	for _, uid := range c.userIDs {
		_ = db.DB.Delete(&user.User{}, uid).Error
	}

	// Delete groups
	for _, gid := range c.groupIDs {
		_ = db.DB.Delete(&group.Group{}, gid).Error
	}

	return nil
}

// PerformanceTimer measures test execution time
type PerformanceTimer struct {
	startTime time.Time
	endTime   time.Time
	name      string
}

// NewPerformanceTimer creates a new performance timer
func NewPerformanceTimer(name string) *PerformanceTimer {
	return &PerformanceTimer{
		name:      name,
		startTime: time.Now(),
	}
}

// Stop stops the timer and returns duration
func (t *PerformanceTimer) Stop() time.Duration {
	t.endTime = time.Now()
	return t.endTime.Sub(t.startTime)
}

// GetDuration returns the duration
func (t *PerformanceTimer) GetDuration() time.Duration {
	if t.endTime.IsZero() {
		return time.Since(t.startTime)
	}
	return t.endTime.Sub(t.startTime)
}

// Report prints a performance report
func (t *PerformanceTimer) Report() string {
	duration := t.GetDuration()
	return fmt.Sprintf("%s took %v", t.name, duration)
}
