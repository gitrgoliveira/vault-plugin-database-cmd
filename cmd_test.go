package cmd

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToMap(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		expected map[string]string
	}{
		{
			name: "all exported params present",
			config: map[string]interface{}{
				"custom_field": "custom_value",
				"username":     "user",
				"password":     "pass",
				"certificate":  "cert",
				"other":        "ignored",
			},
			expected: map[string]string{
				"root_custom_field": "custom_value",
				"root_username":     "user",
				"root_password":     "pass",
				"root_certificate":  "cert",
			},
		},
		{
			name: "some exported params present",
			config: map[string]interface{}{
				"username": "user",
			},
			expected: map[string]string{
				"root_username": "user",
			},
		},
		{
			name:     "no exported params present",
			config:   map[string]interface{}{"other": "value"},
			expected: map[string]string{},
		},
		{
			name: "non-string values ignored",
			config: map[string]interface{}{
				"username": 123,
			},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &cmd{RawConfig: tt.config}
			result := c.ToMap()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReplaceVars(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]string
		tpl      string
		expected string
	}{
		{
			name: "replace single var",
			params: map[string]string{
				"foo": "bar",
			},
			tpl:      "hello {{foo}}",
			expected: "hello bar",
		},
		{
			name: "replace multiple vars",
			params: map[string]string{
				"foo": "bar",
				"baz": "qux",
			},
			tpl:      "{{foo}} says {{baz}}",
			expected: "bar says qux",
		},
		{
			name:     "no vars to replace",
			params:   map[string]string{"foo": "bar"},
			tpl:      "hello world",
			expected: "hello world",
		},
		{
			name:     "empty params",
			params:   map[string]string{},
			tpl:      "hello {{foo}}",
			expected: "hello {{foo}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceVars(tt.params, tt.tpl)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetShell(t *testing.T) {
	shell, flag := getShell()
	if runtime.GOOS == "windows" {
		assert.Equal(t, "cmd.exe", shell)
		assert.Equal(t, "/C", flag)
	} else {
		assert.Equal(t, "/bin/bash", shell)
		assert.Equal(t, "-c", flag)
	}
}
