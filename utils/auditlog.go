package utils

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/models"
)

func LogAudit(c *gin.Context, userID uint, action, resourceType string, resourceID uint, oldObj interface{}, newObj interface{}, description *string) error {
	var oldDataJSON, newDataJSON []byte
	var err error

	if oldObj != nil {
		oldDataJSON, err = json.Marshal(oldObj)
		if err != nil {
			return err
		}
	}
	if newObj != nil {
		newDataJSON, err = json.Marshal(newObj)
		if err != nil {
			return err
		}
	}

	input_description := ""
	if description != nil {
		input_description = *description
	}
	auditLog := models.AuditLog{
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		OldData:      oldDataJSON,
		NewData:      newDataJSON,
		IPAddress:    c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
		Description:  input_description,
	}

	return db.DB.Create(&auditLog).Error
}
