package mps

var (
	ErrInsufficientMPSQuota = "insufficient MPS quota"
	ErrInvalidMPSRequest    = "invalid MPS request"
	ErrConflictingGPUAndMPS = "cannot request both dedicated GPU and MPS"
	ErrNegativeMPSValue     = "MPS value cannot be negative"
)
