package journal

// Status is the lifecycle state of a journaled request.
type Status string

// Known journal request statuses.
const (
	Status_Running   Status = "running"
	Status_Completed Status = "completed"
	Status_Failed    Status = "failed"
)

// String returns the canonical status name.
func (s Status) String() string {
	return string(s)
}
