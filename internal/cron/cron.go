package cron

import (
	"log"
	"time"

	"github.com/linskybing/platform-go/internal/application"
)

func StartCleanupTask(auditService *application.AuditService) {
	go func() {
		log.Println("Starting background cleanup task (retention: 30 days)")

		// Run immediately on startup
		if err := auditService.CleanupOldLogs(30); err != nil {
			log.Printf("Failed to cleanup old audit logs: %v", err)
		}

		// Then run every 24 hours
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			log.Println("Running scheduled audit log cleanup...")
			if err := auditService.CleanupOldLogs(30); err != nil {
				log.Printf("Failed to cleanup old audit logs: %v", err)
			} else {
				log.Println("Audit log cleanup completed successfully")
			}
		}
	}()
}
