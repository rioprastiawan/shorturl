package setup

import (
	"encoding/json"
	"testing"
)

// TestSettingTrueDecodes pins the shape written to app_settings. The column is
// JSONB and Status decodes it as a boolean, so writing the string "true"
// instead of the literal true would leave the wizard permanently reopenable.
func TestSettingTrueDecodes(t *testing.T) {
	var completed bool
	if err := json.Unmarshal(settingTrue, &completed); err != nil {
		t.Fatalf("unmarshal settingTrue: %v", err)
	}
	if !completed {
		t.Errorf("settingTrue decoded to %v, want true", completed)
	}
}

// TestSettingValueInterpretation documents how Status reads the stored flag,
// including the values that must not count as completed.
func TestSettingValueInterpretation(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"literal true", "true", true},
		{"literal false", "false", false},
		{"null", "null", false},
		{"string true is not a boolean", `"true"`, false},
		{"object", `{"completed":true}`, false},
		{"malformed", "not json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var completed bool
			got := json.Unmarshal([]byte(tt.value), &completed) == nil && completed
			if got != tt.want {
				t.Errorf("value %s read as completed=%v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
