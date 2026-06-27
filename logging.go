package jpf

import (
	"time"
)

// ModelLoggingInfo contains all information about a model interaction to be logged.
// It includes input messages, output messages, usage statistics, and any error that occurred.
type ModelLoggingInfo struct {
	InputMessages []Message
	ResultMessage AssistantMessage
	Usage         Usage
	Err           error
	Duration      time.Duration
}

// ModelLogger specifies a method of logging a call to a model.
type ModelLogger interface {
	ModelLog(ModelLoggingInfo) error
}
