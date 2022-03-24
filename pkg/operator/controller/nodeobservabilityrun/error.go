package nodeobservabilityruncontroller

import "fmt"

type NodeObservabilityRunError struct {
	HttpCode int
	Msg      string
}

func (e NodeObservabilityRunError) Error() string {
	return fmt.Sprintf("Code: %d, Error: %s", e.HttpCode, e.Msg)
}

func IsNodeObservabilityRunErrorRetriable(err error) bool {
	e, ok := err.(NodeObservabilityRunError)
	if !ok {
		return false
	}
	if e.HttpCode == 500 {
		return true
	}
	return false
}
