package nodeobservabilityruncontroller

import "fmt"

type NodeObservabilityError struct {
	HttpCode int
	Msg      string
}

func (e NodeObservabilityError) Error() string {
	return fmt.Sprintf("Code: %d, Error: %s", e.HttpCode, e.Msg)
}

func IsNodeObservabilityErrorRetriable(err error) bool {
	e, ok := err.(NodeObservabilityError)
	if !ok {
		return false
	}
	if e.HttpCode == 500 {
		return true
	}
	return false
}
