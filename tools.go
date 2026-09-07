package jpf

import (
	"context"
	"errors"
	"fmt"
	"maps"
)

// ToolSchema specifies how an LLM should call a tool.
type ToolSchema struct {
	Name        string
	Description string
	Params      []ToolParam
}

// ToolParamType defines a type for a parameter of a tool call.
type ToolParamType uint8

const (
	ToolParamInt ToolParamType = iota
	ToolParamFloat
	ToolParamString
)

// ToolParam describes a parameter that an agent may pass to a tool.
type ToolParam struct {
	Name        string
	Description string
	Type        ToolParamType
	Required    bool
}

// ToolArgs are the arguments an LLM has tried to call a tool with.
type ToolArgs map[string]any

// Tool describes a tool that is available to the agent,
// encompassing both directly executable and deferred tools.
type Tool struct {
	// How should the LLM call the tool?
	Schema ToolSchema
	// How should the agent framework run the tool.
	// If not specified, the agent framework will break the loop and defer.
	Call func(context.Context, ToolArgs) (ToolResult, error)
}

// DeferredToolCall describes a call to a named tool that the agent
// framework cannot run, so it has deferred to your code.
type DeferredToolCall struct {
	ToolName string
	CallID   string
	Args     ToolArgs
}

// ToolResult is what a Tool.Call returns when it is run by the agent framework.
type ToolResult struct {
	Content string
}

// DeferredCallResult is what results from your code (not the agent framework)
// calling the tool.
type DeferredCallResult struct {
	CallID  string
	Content string
	Err     error
}

// Takes the raw arguments (from json decoding) and
// modifies / validates them in-place to make them work for the schema.
// For example, converts floats to ints when the schema specifies int.
// Does NOT check for extra args the llm supplied that were not asked for.
func (args ToolArgs) AlignWithSchema(schema ToolSchema) error {
	errs := make([]error, 0)
	for _, schemaArg := range schema.Params {
		val, ok := args[schemaArg.Name]
		if !ok {
			if schemaArg.Required {
				errs = append(errs, fmt.Errorf("argument '%s' is required but was not provided", schemaArg.Name))
			}
			continue
		}
		switch schemaArg.Type {
		case ToolParamFloat:
			if _, ok := val.(float64); !ok {
				errs = append(errs, fmt.Errorf("argument '%s' must be a float but got %T", schemaArg.Name, val))
			}
		case ToolParamInt:
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
		case ToolParamString:
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

func (args ToolArgs) Clone() ToolArgs {
	return maps.Clone(args)
}

func (args ToolArgs) RequiredString(key string) string {
	return args[key].(string)
}

func (args ToolArgs) RequiredInt(key string) int {
	return args[key].(int)
}

func (args ToolArgs) RequiredFloat(key string) float64 {
	return args[key].(float64)
}

func (args ToolArgs) RequiredBool(key string) bool {
	return args[key].(bool)
}

func (args ToolArgs) OptionalString(key string, defaultVal string) (string, bool) {
	val, ok := args[key]
	if !ok {
		return defaultVal, false
	}
	return val.(string), true
}

func (args ToolArgs) OptionalInt(key string, defaultVal int) (int, bool) {
	val, ok := args[key]
	if !ok {
		return defaultVal, false
	}
	return val.(int), true
}

func (args ToolArgs) OptionalFloat(key string, defaultVal float64) (float64, bool) {
	val, ok := args[key]
	if !ok {
		return defaultVal, false
	}
	return val.(float64), true
}

func (args ToolArgs) OptionalBool(key string, defaultVal bool) (bool, bool) {
	val, ok := args[key]
	if !ok {
		return defaultVal, false
	}
	return val.(bool), true
}
