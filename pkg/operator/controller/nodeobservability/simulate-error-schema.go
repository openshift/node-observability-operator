package nodeobservabilitycontroller

const (
	crObj  = "clusterrole"
	crbObj = "clusterrolebinding"
)

// ErrTestObject - simple struct used to inject errors for testing
type ErrTestObject struct {
	Enabled  bool
	Set      map[string]bool
	NotFound map[string]bool
}
