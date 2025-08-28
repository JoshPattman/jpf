package jpf

import (
	"errors"
	"reflect"
	"testing"
)

func TestNewJsonResponseDecoder_Panic(t *testing.T) {
	// Test that it panics with invalid types
	panics := func(f func()) (didPanic bool) {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
			}
		}()
		f()
		return false
	}

	tests := []struct {
		name       string
		createFunc func()
		wantPanic  bool
	}{
		{
			name: "slice type",
			createFunc: func() {
				NewJsonResponseDecoder[[]string]()
			},
			wantPanic: true,
		},
		{
			name: "int type",
			createFunc: func() {
				NewJsonResponseDecoder[int]()
			},
			wantPanic: true,
		},
		{
			name: "map with int keys",
			createFunc: func() {
				NewJsonResponseDecoder[map[int]string]()
			},
			wantPanic: true,
		},
		{
			name: "struct type",
			createFunc: func() {
				NewJsonResponseDecoder[struct{ Name string }]()
			},
			wantPanic: false,
		},
		{
			name: "map with string keys",
			createFunc: func() {
				NewJsonResponseDecoder[map[string]interface{}]()
			},
			wantPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := panics(tt.createFunc)
			if got != tt.wantPanic {
				t.Errorf("NewJsonResponseDecoder() panic = %v, want %v", got, tt.wantPanic)
			}
		})
	}
}

func TestJsonResponseDecoder_ParseResponseText(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name     string
		response string
		want     Person
		wantErr  bool
		errIs    error
	}{
		{
			name:     "valid json",
			response: `{"name": "Josh", "age": 30}`,
			want:     Person{Name: "Josh", Age: 30},
			wantErr:  false,
		},
		{
			name:     "valid json with prefix and suffix",
			response: `Some text before {"name": "Josh", "age": 30} and some after`,
			want:     Person{Name: "Josh", Age: 30},
			wantErr:  false,
		},
		{
			name:     "valid json with nested objects",
			response: `The model says: {"name": "Josh", "age": 30, "details": {"job": "developer"}}`,
			want:     Person{Name: "Josh", Age: 30},
			wantErr:  false,
		},
		{
			name:     "no json object",
			response: `This is just plain text with no JSON object`,
			want:     Person{},
			wantErr:  true,
			errIs:    ErrInvalidResponse,
		},
		{
			name:     "malformed json",
			response: `{"name": "Josh", "age": "thirty"}`,
			want:     Person{},
			wantErr:  true,
			errIs:    ErrInvalidResponse,
		},
		{
			name:     "json array instead of object",
			response: `["Josh", 30]`,
			want:     Person{},
			wantErr:  true,
			errIs:    ErrInvalidResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := NewJsonResponseDecoder[Person]()
			got, err := decoder.ParseResponseText(tt.response)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseResponseText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// If expecting a specific error, check it
			if tt.errIs != nil && err != nil && !errors.Is(err, tt.errIs) {
				t.Errorf("ParseResponseText() error = %v, want error containing %v", err, tt.errIs)
			}

			// Check result
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseResponseText() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJsonResponseDecoder_WithMapType(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     map[string]interface{}
		wantErr  bool
	}{
		{
			name:     "valid json",
			response: `{"name": "Josh", "age": 30, "isActive": true}`,
			want: map[string]interface{}{
				"name":     "Josh",
				"age":      float64(30), // JSON numbers are float64 by default
				"isActive": true,
			},
			wantErr: false,
		},
		{
			name:     "nested json",
			response: `{"user": {"name": "Josh", "age": 30}, "settings": {"theme": "dark"}}`,
			want: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "Josh",
					"age":  float64(30),
				},
				"settings": map[string]interface{}{
					"theme": "dark",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoder := NewJsonResponseDecoder[map[string]interface{}]()
			got, err := decoder.ParseResponseText(tt.response)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseResponseText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check result
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseResponseText() got = %v, want %v", got, tt.want)
			}
		})
	}
}
