package jpf

import (
	"encoding/json"
	"io"
)

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
