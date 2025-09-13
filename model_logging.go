package jpf

import (
	"time"
)

// NewLoggingModel wraps a Model with logging functionality.
// It logs all interactions with the model using the provided ModelLogger.
// Each model call is logged with input messages, output messages, usage statistics, and timing information.
func NewLoggingModel(model Model, logger ModelLogger) Model {
	return &loggingModel{
		model:  model,
		logger: logger,
	}
}

type loggingModel struct {
	logger ModelLogger
	model  Model
}

// Respond implements Model.
func (l *loggingModel) Respond(msgs []Message) ([]Message, Message, Usage, error) {
	tStart := time.Now()
	aux, final, us, err := l.model.Respond(msgs)
	dur := time.Since(tStart)
	lmp := ModelLoggingInfo{
		Messages:             msgs,
		ResponseAuxMessages:  aux,
		ResponseFinalMessage: final,
		Usage:                us,
		Err:                  err,
		Duration:             dur,
	}
	logErr := l.logger.ModelLog(lmp)
	if err == nil {
		err = logErr
	}
	return aux, final, us, err
}

// Tokens implements Model.
func (l *loggingModel) Tokens() (int, int) {
	return l.model.Tokens()
}
