package jpf

import (
	"encoding/json"
	"fmt"
	"io"
)

type LoggingModelBuilder struct {
	model *loggingModel
}

func BuildLoggingModel(dst io.Writer, model Model) *LoggingModelBuilder {
	return &LoggingModelBuilder{
		model: &loggingModel{
			model:   model,
			dst:     dst,
			logFunc: LogWithJson,
		},
	}
}

func (lmb *LoggingModelBuilder) Validate() (Model, error) {
	if lmb.model.dst == nil {
		return nil, fmt.Errorf("must have a non nil destinationm for logging model")
	}
	if lmb.model.logFunc == nil {
		return nil, fmt.Errorf("must have a non nil logfunc for logging model")
	}
	return lmb.model, nil
}

func (lmb *LoggingModelBuilder) WithLogFunc(logFunc func(ModelLoggingInfo, io.Writer) error) *LoggingModelBuilder {
	lmb.model.logFunc = logFunc
	return lmb
}

type ModelLoggingInfo struct {
	messages             []Message
	responseAuxMessages  []Message
	responseFinalMessage Message
	usage                Usage
	err                  error
}

func messageToLoggingJson(msg Message) any {
	return map[string]any{
		"role":    msg.Role.String(),
		"content": msg.Content,
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

func LogWithJson(lmp ModelLoggingInfo, dst io.Writer) error {
	res := map[string]any{
		"messages":       messagesToLoggingJson(lmp.messages),
		"aux_responses":  messagesToLoggingJson(lmp.responseAuxMessages),
		"final_response": messageToLoggingJson(lmp.responseFinalMessage),
		"usage":          usageToLoggingJson(lmp.usage),
	}
	if lmp.err != nil {
		res["error"] = lmp.err.Error()
	}
	bs, err := json.Marshal(res)
	if err != nil {
		return err
	}
	_, err = dst.Write(bs)
	return err
}

type loggingModel struct {
	dst     io.Writer
	logFunc func(ModelLoggingInfo, io.Writer) error
	model   Model
}

// Respond implements Model.
func (l *loggingModel) Respond(msgs []Message) ([]Message, Message, Usage, error) {
	aux, final, us, err := l.model.Respond(msgs)
	lmp := ModelLoggingInfo{
		messages:             msgs,
		responseAuxMessages:  aux,
		responseFinalMessage: final,
		usage:                us,
		err:                  err,
	}
	logErr := l.logFunc(lmp, l.dst)
	if err == nil {
		err = logErr
	}
	return aux, final, us, err
}

// Tokens implements Model.
func (l *loggingModel) Tokens() (int, int) {
	return l.model.Tokens()
}
