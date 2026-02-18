package loggers

import "github.com/JoshPattman/jpf"

// Logs calls made to the model to a slog-style logging function.
// Can optionally log the model messages too (this is very spammy).
func NewSlog(logFunc func(string, ...any), logMessages bool) jpf.ModelLogger {
	return &slogModelLogger{
		logFunc:     logFunc,
		logMessages: logMessages,
	}
}

type slogModelLogger struct {
	logFunc     func(string, ...any)
	logMessages bool
}

func (ml *slogModelLogger) ModelLog(mli jpf.ModelLoggingInfo) error {
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
