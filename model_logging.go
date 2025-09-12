package jpf

import (
	"encoding/json"
	"io"
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

// ModelLoggingInfo contains all information about a model interaction to be logged.
// It includes input messages, output messages, usage statistics, and any error that occurred.
type ModelLoggingInfo struct {
	Messages             []Message
	ResponseAuxMessages  []Message
	ResponseFinalMessage Message
	Usage                Usage
	Err                  error
	Duration             time.Duration
}

// ModelLogger specifies a method of logging a call to a model.
type ModelLogger interface {
	ModelLog(ModelLoggingInfo) error
}

// NewJsonModelLogger creates a ModelLogger that outputs logs in JSON format.
// The logs are written to the provided io.Writer, with each log entry
// being a JSON object containing the model interaction details.
func NewJsonModelLogger(to io.Writer) ModelLogger {
	return &jsonModelLogger{enc: json.NewEncoder(to)}
}

type jsonModelLogger struct {
	enc *json.Encoder
}

// ModelLog implements ModelLogger.
func (j *jsonModelLogger) ModelLog(lmp ModelLoggingInfo) error {
	res := map[string]any{
		"messages":       messagesToLoggingJson(lmp.Messages),
		"aux_responses":  messagesToLoggingJson(lmp.ResponseAuxMessages),
		"final_response": messageToLoggingJson(lmp.ResponseFinalMessage),
		"usage":          usageToLoggingJson(lmp.Usage),
		"duration":       lmp.Duration.String(),
	}
	if lmp.Err != nil {
		res["error"] = lmp.Err.Error()
	}
	return j.enc.Encode(res)
}

func messageToLoggingJson(msg Message) any {
	return map[string]any{
		"role":       msg.Role.String(),
		"content":    msg.Content,
		"num_images": len(msg.Images),
	}
}

func messagesToLoggingJson(msgs []Message) any {
	res := make([]any, 0)
	for _, m := range msgs {
		res = append(res, messageToLoggingJson(m))
	}
	return res
}

func usageToLoggingJson(usage Usage) any {
	return map[string]any{
		"input_tokens":  usage.InputTokens,
		"output_tokens": usage.OutputTokens,
	}
}

// Logs calls made to the model to a slog-style logging function.
// Can optionally log the model messages too (this is very spammy).
func NewSlogModelLogger(logFunc func(string, ...any), logMessages bool) ModelLogger {
	return &slogModelLogger{
		logFunc:     logFunc,
		logMessages: logMessages,
	}
}

type slogModelLogger struct {
	logFunc     func(string, ...any)
	logMessages bool
}

func (ml *slogModelLogger) ModelLog(mli ModelLoggingInfo) error {
	args := []any{}
	args = append(args, "input_tokens", mli.Usage.InputTokens)
	args = append(args, "output_tokens", mli.Usage.OutputTokens)
	args = append(args, "time_taken", mli.Duration.String())
	if mli.Err != nil {
		args = append(args, "error", mli.Err.Error())
	}

	if ml.logMessages {
		args = append(args, "input_messages", mli.Messages)
		args = append(args, "output_aux_messages", mli.ResponseAuxMessages)
		args = append(args, "output_final_message", mli.ResponseFinalMessage)
	}

	ml.logFunc("model_call", args...)
	return nil
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
