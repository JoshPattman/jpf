package agents

import (
	"context"
	"errors"
	"fmt"

	"github.com/JoshPattman/jpf"
)

type Tool struct {
	// How and when can the agent call the tool, and what it should expect it to do.
	Schema jpf.ToolSchema
	// How should the agent framework run the tool.
	// If not specified, the agent framework will break the loop and defer.
	Call func(context.Context, map[string]any) (string, error)
}

// Fetches the arg from the tool args, must only be called on required args defined int he schema.
func RequiredArg[T any](args map[string]any, name string) T {
	return args[name].(T)
}

// Fetches the arg from the tool args, must only be called on optional args defined int he schema.
func OptionalArg[T any](args map[string]any, name string) (T, bool) {
	a, ok := args[name]
	if !ok {
		return *new(T), false
	}
	return a.(T), true
}

// Takes a raw dictionary that you get from json decoding and
// modifies / validates it in place to make it work for the schema.
// Does NOT check for extra args the llm supplied that were not asked for.
func validateAndFixArgsForSchema(args map[string]any, schema jpf.ToolSchema) error {
	errs := make([]error, 0)
	for _, schemaArg := range schema.Args {
		val, ok := args[schemaArg.Name]
		if !ok {
			if schemaArg.Required {
				errs = append(errs, fmt.Errorf("argument '%s' is required but was not provided", schemaArg.Name))
			}
			continue
		}
		switch schemaArg.Type {
		case jpf.ToolArgFloat:
			if _, ok := val.(float64); !ok {
				errs = append(errs, fmt.Errorf("argument '%s' must be a float but got %T", schemaArg.Name, val))
			}
		case jpf.ToolArgInt:
			switch val := val.(type) {
			case float64:
				if float64(int(val)) != val {
					errs = append(errs, fmt.Errorf("argument '%s' must be an int but got float", schemaArg.Name))
				} else {
					args[schemaArg.Name] = int(val)
				}
			case int:
			default:
				errs = append(errs, fmt.Errorf("argument '%s' must be an int but got %T", schemaArg.Name, val))
			}
		case jpf.ToolArgString:
			if _, ok := val.(string); !ok {
				errs = append(errs, fmt.Errorf("argument '%s' must be a string but got %T", schemaArg.Name, val))
			}
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
