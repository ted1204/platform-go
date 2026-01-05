package mps

// ProjectMPSQuota represents MPS quota information for a project
type ProjectMPSQuota struct {
	ProjectID     uint
	TotalMPSUnits int // Total MPS units available
	UsedMPSUnits  int // Currently allocated MPS units
	// MPSLimit field has been removed
}

// AvailableMPS returns the remaining MPS units available
func (q *ProjectMPSQuota) AvailableMPS() int {
	return q.TotalMPSUnits - q.UsedMPSUnits
}

// CanAllocate checks if the requested MPS can be allocated
func (q *ProjectMPSQuota) CanAllocate(requested int) bool {
	return q.AvailableMPS() >= requested
}

// Allocate allocates MPS units if available
func (q *ProjectMPSQuota) Allocate(requested int) error {
	if requested < 0 {
		return nil
	}
	if !q.CanAllocate(requested) {
		return nil
	}
	q.UsedMPSUnits += requested
	return nil
}

// Release releases allocated MPS units
func (q *ProjectMPSQuota) Release(amount int) error {
	if amount < 0 {
		return nil
	}
	if amount > q.UsedMPSUnits {
		amount = q.UsedMPSUnits
	}
	q.UsedMPSUnits -= amount
	return nil
}

// UsagePercent returns the percentage of MPS quota used
func (q *ProjectMPSQuota) UsagePercent() float64 {
	if q.TotalMPSUnits == 0 {
		return 0
	}
	return float64(q.UsedMPSUnits) / float64(q.TotalMPSUnits) * 100
}
