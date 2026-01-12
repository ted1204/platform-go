//go:build !integration
// +build !integration

package migrations

// Stub package to satisfy imports while real migrations are not present.
// It is excluded when running integration tests (build tag "integration").
// Remove this file once the real `internal/migrations` package is restored.

// Run is a no-op placeholder used for unit-test isolation.
func Run() error { return nil }
