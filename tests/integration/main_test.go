package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/config"
	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/internal/testutils"
	"github.com/linskybing/platform-go/routes"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "github.com/lib/pq"
)

var (
	cleanup func()
	router  *gin.Engine
)

func TestMain(m *testing.M) {
	sqlDB, cleanup := testutils.SetupPostgresForIntegration()
	defer cleanup()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		Logger: logger.New(
			log.New(io.Discard, "", log.LstdFlags),
			logger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  logger.Silent,
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		),
	})
	if err != nil {
		log.Fatal(err)
	}
	config.LoadConfig()
	db.InitWithGormDB(gormDB)

	// Gin router
	gin.SetMode(gin.TestMode)
	router = gin.New()
	routes.RegisterRoutes(router)

	// setup
	registerUserForTests("alice", "123456")
	registerUserForTests("test1", "123456")
	registerUserForTests("test2", "123456")

	code := m.Run()
	os.Exit(code)
}

// --- Helper functions ---
// doRequest is a generalized helper to make HTTP requests in tests.
// Supports:
// - body as url.Values -> form-urlencoded
// - body as any other struct/map -> JSON
// - nil body -> GET/DELETE with query parameters included in path
func doRequest(t *testing.T, method, path string, token string, body interface{}, expectStatus int) *httptest.ResponseRecorder {
	var req *http.Request

	switch v := body.(type) {
	case url.Values: // form-urlencoded
		req = httptest.NewRequest(method, path, strings.NewReader(v.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	case nil: // nil body, assume parameters are already in path
		req = httptest.NewRequest(method, path, nil)
	default: // JSON body
		reqBody, err := json.Marshal(body)
		require.NoError(t, err)
		req = httptest.NewRequest(method, path, bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if expectStatus != 0 {
		require.Equal(t, expectStatus, w.Code,
			fmt.Sprintf("expected %d, got %d, body=%s", expectStatus, w.Code, w.Body.String()))
	}

	return w
}

func registerUserForTests(username, password string) {
	w := httptest.NewRecorder()
	reqBody := fmt.Sprintf(`{"username":"%s","password":"%s"}`, username, password)
	req, _ := http.NewRequest("POST", "/register", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
}
