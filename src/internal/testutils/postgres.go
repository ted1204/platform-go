package testutils

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"

	"embed"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

//
//go:embed schema.sql
var schemaFS embed.FS

func SetupPostgresForIntegration() (*sql.DB, func()) {
	// Check if an external DB DSN is provided
	if dsn := os.Getenv("TEST_DB_DSN"); dsn != "" {
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			log.Fatal(err)
		}
		if err := db.Ping(); err != nil {
			log.Fatal(err)
		}

		// Apply schema
		schemaBytes, err := schemaFS.ReadFile("schema.sql")
		if err != nil {
			log.Fatal(err)
		}
		if _, err := db.Exec(string(schemaBytes)); err != nil {
			log.Fatal(err)
		}

		return db, func() {
			_ = db.Close()
		}
	}

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image: "postgres:15",
		Env: map[string]string{
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_USER":     "test",
			"POSTGRES_DB":       "platform",
		},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForLog("database system is ready to accept connections").WithStartupTimeout(30 * time.Second),
	}

	pg, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatal(err)
	}

	host, err := pg.Host(ctx)
	if err != nil {
		log.Fatal(err)
	}
	port, err := pg.MappedPort(ctx, "5432")
	if err != nil {
		log.Fatal(err)
	}

	dsn := fmt.Sprintf("postgres://test:test@%s:%s/platform?sslmode=disable", host, port.Port())
	os.Setenv("DATABASE_URL", dsn)

	// retry db connect
	var db *sql.DB
	for i := 0; i < 10; i++ {
		db, err = sql.Open("postgres", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				break
			}
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		log.Fatal(err)
	}

	schemaBytes, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := db.Exec(string(schemaBytes)); err != nil {
		log.Fatal(err)
	}

	cleanup := func() {
		_ = db.Close()
		_ = pg.Terminate(ctx)
	}

	return db, cleanup
}
