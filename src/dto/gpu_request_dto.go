package dto

type CreateGPURequestDTO struct {
	Type                string `json:"type" binding:"required,oneof=quota access"`
	RequestedQuota      *int   `json:"requested_quota,omitempty"`
	RequestedAccessType *string `json:"requested_access_type,omitempty"`
	Reason              string `json:"reason" binding:"required"`
}

type UpdateGPURequestStatusDTO struct {
	Status string `json:"status" binding:"required,oneof=approved rejected"`
}
