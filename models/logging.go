package models

import (
	"context"
	"time"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/internal/utils"
)

// Log wraps a Model with logging functionality.
// It logs all interactions with the model using the provided ModelLogger.
// Each model call is logged with input messages, output messages, usage statistics, and timing information.
func Log(model jpf.Model, logger jpf.ModelLogger) jpf.Model {
	return &loggingModel{
		model:  model,
		logger: logger,
	}
}

type loggingModel struct {
	logger jpf.ModelLogger
	model  jpf.Model
}

// Respond implements Model.
func (l *loggingModel) Respond(ctx context.Context, msgs []jpf.Message) (jpf.ModelResponse, error) {
	tStart := time.Now()
	resp, err := l.model.Respond(ctx, msgs)
	dur := time.Since(tStart)
	lmp := jpf.ModelLoggingInfo{
		Messages:             msgs,
		ResponseFinalMessage: resp.Message,
		Usage:                resp.Usage,
		Err:                  err,
		Duration:             dur,
	}
	logErr := l.logger.ModelLog(lmp)
	if err != nil {
		// There was an error with the original mode, do nothing
	} else if logErr != nil {
		// There was not an original model error, but we got a logging error
		err = utils.Wrap(logErr, "failed to execute logging")
	}
	return resp, err
}
