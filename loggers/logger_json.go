package loggers

import (
	"encoding/json"
	"io"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/internal/utils"
)

// NewJson creates a ModelLogger that outputs logs in JSON format.
// The logs are written to the provided io.Writer, with each log entry
// being a JSON object containing the model interaction details.
func NewJson(to io.Writer) jpf.ModelLogger {
	return &jsonModelLogger{enc: json.NewEncoder(to)}
}

type jsonModelLogger struct {
	enc *json.Encoder
}

// ModelLog implements ModelLogger.
func (j *jsonModelLogger) ModelLog(lmp jpf.ModelLoggingInfo) error {
	res := map[string]any{
		"messages":       messagesToLoggingJson(lmp.InputMessages),
		"final_response": messageToLoggingJson(lmp.ResultMessage),
		"usage":          usageToLoggingJson(lmp.Usage),
		"duration":       lmp.Duration.String(),
	}
	if lmp.Err != nil {
		res["error"] = lmp.Err.Error()
	}
	err := j.enc.Encode(res)
	if err != nil {
		return utils.Wrap(err, "failed to encode information to json")
	}
	return nil
}

func messageToLoggingJson(msg jpf.Message) any {
	switch msg := msg.(type) {
	case jpf.UserMessage:
		return map[string]any{
			"role":       "user",
			"content":    msg.Content,
			"num_images": len(msg.Images),
		}
	case jpf.AssistantMessage:
		return map[string]any{
			"role":    "assistant",
			"content": msg.Content,
		}
	case jpf.DeveloperMessage:
		return map[string]any{
			"role":    "developer",
			"content": msg.Content,
		}
	case jpf.SystemMessage:
		return map[string]any{
			"role":    "system",
			"content": msg.Content,
		}
	default:
		panic("unreachable")
	}
}

func messagesToLoggingJson(msgs []jpf.Message) any {
	res := make([]any, 0)
	for _, m := range msgs {
		res = append(res, messageToLoggingJson(m))
	}
	return res
}

func usageToLoggingJson(usage jpf.Usage) any {
	return map[string]any{
		"input_tokens":  usage.InputTokens,
		"output_tokens": usage.OutputTokens,
	}
}
