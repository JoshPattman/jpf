package jpf

import (
	"testing"
)

func TestValidateAndFixArgsForSchema(t *testing.T) {
	tests := []struct {
		name     string
		schema   ToolSchema
		args     ToolArgs
		wantErr  bool
		wantArgs map[string]any
	}{
		{
			name: "valid string arg",
			schema: ToolSchema{Params: []ToolParam{
				{Name: "s", Type: ToolParamString},
			}},
			args:     map[string]any{"s": "hello"},
			wantErr:  false,
			wantArgs: map[string]any{"s": "hello"},
		},
		{
			name: "valid float arg",
			schema: ToolSchema{Params: []ToolParam{
				{Name: "f", Type: ToolParamFloat},
			}},
			args:     map[string]any{"f": 1.5},
			wantErr:  false,
			wantArgs: map[string]any{"f": 1.5},
		},
		{
			name: "valid int arg is converted from float64",
			schema: ToolSchema{Params: []ToolParam{
				{Name: "i", Type: ToolParamInt},
			}},
			args:     map[string]any{"i": float64(3)},
			wantErr:  false,
			wantArgs: map[string]any{"i": 3},
		},
		{
			name: "int arg supplied as native int passes",
			schema: ToolSchema{Params: []ToolParam{
				{Name: "i", Type: ToolParamInt},
			}},
			args:     map[string]any{"i": int(3)},
			wantErr:  false,
			wantArgs: map[string]any{"i": 3},
		},
		{
			name: "int arg with fractional float64 errors",
			schema: ToolSchema{Params: []ToolParam{
				{Name: "i", Type: ToolParamInt},
			}},
			args:    map[string]any{"i": 3.5},
			wantErr: true,
		},
		{
			name: "missing arg errors",
			schema: ToolSchema{Params: []ToolParam{
				{Name: "s", Type: ToolParamString},
			}},
			args:    map[string]any{},
			wantErr: true,
		},
		{
			name: "wrong type string errors",
			schema: ToolSchema{Params: []ToolParam{
				{Name: "s", Type: ToolParamString},
			}},
			args:    map[string]any{"s": 5.0},
			wantErr: true,
		},
		{
			name: "wrong type float errors",
			schema: ToolSchema{Params: []ToolParam{
				{Name: "f", Type: ToolParamFloat},
			}},
			args:    map[string]any{"f": "not a float"},
			wantErr: true,
		},
		{
			name: "wrong type int errors",
			schema: ToolSchema{Params: []ToolParam{
				{Name: "i", Type: ToolParamInt},
			}},
			args:    map[string]any{"i": "not an int"},
			wantErr: true,
		},
		{
			name: "multiple errors are joined",
			schema: ToolSchema{Params: []ToolParam{
				{Name: "s", Type: ToolParamString},
				{Name: "i", Type: ToolParamInt},
			}},
			args:    map[string]any{"s": 5.0, "i": "not an int"},
			wantErr: true,
		},
		{
			name: "extra unrecognised args are ignored",
			schema: ToolSchema{Params: []ToolParam{
				{Name: "s", Type: ToolParamString},
			}},
			args:     map[string]any{"s": "hello", "extra": "ignored"},
			wantErr:  false,
			wantArgs: map[string]any{"s": "hello", "extra": "ignored"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.AlignWithSchema(tt.schema)
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
