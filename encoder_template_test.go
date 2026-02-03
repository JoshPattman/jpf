package jpf

import (
	"reflect"
	"testing"
)

func TestTemplateEncoder_BuildInputMessages(t *testing.T) {
	type Person struct {
		Name string
		Age  int
	}

	tests := []struct {
		name           string
		systemTemplate string
		userTemplate   string
		data           Person
		wantMessages   []Message
		wantErr        bool
	}{
		{
			name:           "both templates",
			systemTemplate: "You are helping {{.Name}} who is {{.Age}} years old.",
			userTemplate:   "Hello, my name is {{.Name}} and I am {{.Age}}.",
			data:           Person{Name: "Josh", Age: 30},
			wantMessages: []Message{
				{Role: SystemRole, Content: "You are helping Josh who is 30 years old."},
				{Role: UserRole, Content: "Hello, my name is Josh and I am 30."},
			},
			wantErr: false,
		},
		{
			name:           "only system template",
			systemTemplate: "You are helping {{.Name}} who is {{.Age}} years old.",
			userTemplate:   "",
			data:           Person{Name: "Josh", Age: 30},
			wantMessages: []Message{
				{Role: SystemRole, Content: "You are helping Josh who is 30 years old."},
			},
			wantErr: false,
		},
		{
			name:           "only user template",
			systemTemplate: "",
			userTemplate:   "Hello, my name is {{.Name}} and I am {{.Age}}.",
			data:           Person{Name: "Josh", Age: 30},
			wantMessages: []Message{
				{Role: UserRole, Content: "Hello, my name is Josh and I am 30."},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoder := NewTemplateEncoder[Person](tt.systemTemplate, tt.userTemplate)
			got, err := encoder.BuildInputMessages(tt.data)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildInputMessages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check messages
			if !reflect.DeepEqual(got, tt.wantMessages) {
				t.Errorf("BuildInputMessages() got = %v, want %v", got, tt.wantMessages)
			}
		})
	}
}

func TestTemplateEncoder_BuildInputMessages_Error(t *testing.T) {
	// Test with invalid template
	encoder := NewTemplateEncoder[struct{}]("{{ .MissingField }}", "")
	_, err := encoder.BuildInputMessages(struct{}{})
	if err == nil {
		t.Errorf("Expected error for missing field in template, got nil")
	}
}

func TestTemplateEncoder_DifferentTypes(t *testing.T) {
	// Test with map type
	t.Run("map type", func(t *testing.T) {
		encoder := NewTemplateEncoder[map[string]string]("Hello {{.name}}", "")
		got, err := encoder.BuildInputMessages(map[string]string{"name": "Josh"})

		if err != nil {
			t.Errorf("BuildInputMessages() error = %v", err)
			return
		}

		expected := []Message{
			{Role: SystemRole, Content: "Hello Josh"},
		}

		if !reflect.DeepEqual(got, expected) {
			t.Errorf("BuildInputMessages() got = %v, want %v", got, expected)
		}
	})

	// Test with primitive type (requires special handling in templates)
	t.Run("string type with dot value", func(t *testing.T) {
		encoder := NewTemplateEncoder[string]("You said: {{.}}", "")
		got, err := encoder.BuildInputMessages("hello world")

		if err != nil {
			t.Errorf("BuildInputMessages() error = %v", err)
			return
		}

		expected := []Message{
			{Role: SystemRole, Content: "You said: hello world"},
		}

		if !reflect.DeepEqual(got, expected) {
			t.Errorf("BuildInputMessages() got = %v, want %v", got, expected)
		}
	})
}
