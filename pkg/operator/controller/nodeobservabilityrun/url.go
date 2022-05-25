package nodeobservabilityruncontroller

import (
	"fmt"
	"strings"
)

type URL interface {
	format(ip, agentName, namespace, path string, port int32) string
}

type url struct{}
type testURL struct{}

func (u *url) format(ip, agentName, namespace, path string, port int32) string {
	hostname := strings.ReplaceAll(ip, ".", "-")
	return fmt.Sprintf("https://%s.%s.%s.svc:%d/%s", hostname, agentName, namespace, port, path)
}

func (u *testURL) format(hostname, agentName, namespace, path string, port int32) string {
	return fmt.Sprintf("https://%s:%d/%s", hostname, port, path)
}
