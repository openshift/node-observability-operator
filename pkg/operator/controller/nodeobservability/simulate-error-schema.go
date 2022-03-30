package nodeobservabilitycontroller

const (
	saObj  = "serviceaccount"
	svcObj = "service"
	sccObj = "securitycontextconstraint"
	crObj  = "clusterrole"
	crbObj = "clusterrolebinding"
	dsObj  = "daemonset"
)

// ErrTestObject - simple struct used to inject errors for testing
type ErrTestObject struct {
	Set      map[string]bool
	NotFound map[string]bool
}
