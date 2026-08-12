package agents

import (
	"testing"

	"github.com/JoshPattman/jpf"
)

func TestValidateAndFixArgsForSchema(t *testing.T) {
	tests := []struct {
		name     string
		schema   jpf.ToolSchema
		args     map[string]any
		wantErr  bool
		wantArgs map[string]any
	}{
		{
			name: "valid string arg",
			schema: jpf.ToolSchema{Args: []jpf.ToolArg{
				{Name: "s", Type: jpf.ToolArgString, Required: true},
			}},
			args:     map[string]any{"s": "hello"},
			wantErr:  false,
			wantArgs: map[string]any{"s": "hello"},
		},
		{
			name: "valid float arg",
			schema: jpf.ToolSchema{Args: []jpf.ToolArg{
				{Name: "f", Type: jpf.ToolArgFloat, Required: true},
			}},
			args:     map[string]any{"f": 1.5},
			wantErr:  false,
			wantArgs: map[string]any{"f": 1.5},
		},
		{
			name: "valid int arg is converted from float64",
			schema: jpf.ToolSchema{Args: []jpf.ToolArg{
				{Name: "i", Type: jpf.ToolArgInt, Required: true},
			}},
			args:     map[string]any{"i": float64(3)},
			wantErr:  false,
			wantArgs: map[string]any{"i": 3},
		},
		{
			name: "int arg supplied as native int passes",
			schema: jpf.ToolSchema{Args: []jpf.ToolArg{
				{Name: "i", Type: jpf.ToolArgInt, Required: true},
			}},
			args:     map[string]any{"i": int(3)},
			wantErr:  false,
			wantArgs: map[string]any{"i": 3},
		},
		{
			name: "int arg with fractional float64 errors",
			schema: jpf.ToolSchema{Args: []jpf.ToolArg{
				{Name: "i", Type: jpf.ToolArgInt, Required: true},
			}},
			args:    map[string]any{"i": 3.5},
			wantErr: true,
		},
		{
			name: "missing required arg errors",
			schema: jpf.ToolSchema{Args: []jpf.ToolArg{
				{Name: "s", Type: jpf.ToolArgString, Required: true},
			}},
			args:    map[string]any{},
			wantErr: true,
		},
		{
			name: "missing optional arg is fine",
			schema: jpf.ToolSchema{Args: []jpf.ToolArg{
				{Name: "s", Type: jpf.ToolArgString, Required: false},
			}},
			args:     map[string]any{},
			wantErr:  false,
			wantArgs: map[string]any{},
		},
		{
			name: "wrong type string errors",
			schema: jpf.ToolSchema{Args: []jpf.ToolArg{
				{Name: "s", Type: jpf.ToolArgString, Required: true},
			}},
			args:    map[string]any{"s": 5.0},
			wantErr: true,
		},
		{
			name: "wrong type float errors",
			schema: jpf.ToolSchema{Args: []jpf.ToolArg{
				{Name: "f", Type: jpf.ToolArgFloat, Required: true},
			}},
			args:    map[string]any{"f": "not a float"},
			wantErr: true,
		},
		{
			name: "wrong type int errors",
			schema: jpf.ToolSchema{Args: []jpf.ToolArg{
				{Name: "i", Type: jpf.ToolArgInt, Required: true},
			}},
			args:    map[string]any{"i": "not an int"},
			wantErr: true,
		},
		{
			name: "multiple errors are joined",
			schema: jpf.ToolSchema{Args: []jpf.ToolArg{
				{Name: "s", Type: jpf.ToolArgString, Required: true},
				{Name: "i", Type: jpf.ToolArgInt, Required: true},
			}},
			args:    map[string]any{"s": 5.0, "i": "not an int"},
			wantErr: true,
		},
		{
			name: "extra unrecognised args are ignored",
			schema: jpf.ToolSchema{Args: []jpf.ToolArg{
				{Name: "s", Type: jpf.ToolArgString, Required: true},
			}},
			args:     map[string]any{"s": "hello", "extra": "ignored"},
			wantErr:  false,
			wantArgs: map[string]any{"s": "hello", "extra": "ignored"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAndFixArgsForSchema(tt.args, tt.schema)
			if tt.wantErr && err == nil {
				t.Fatalf("expected an error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error but got: %v", err)
			}
			if !tt.wantErr {
				if len(tt.args) != len(tt.wantArgs) {
					t.Fatalf("expected args %v, got %v", tt.wantArgs, tt.args)
				}
				for k, v := range tt.wantArgs {
					if tt.args[k] != v {
						t.Fatalf("expected arg %s to be %v (%T), got %v (%T)", k, v, v, tt.args[k], tt.args[k])
					}
				}
			}
		})
	}
}
