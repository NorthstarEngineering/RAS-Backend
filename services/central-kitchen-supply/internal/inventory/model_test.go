package inventory

import "testing"

func TestMovementTypeValidation(t *testing.T) {
	tests := []struct {
		name  string
		value MovementType
		want  bool
	}{
		{"receipt", TypeIn, true},
		{"issue", TypeOut, true},
		{"adjustment", TypeAdjustment, true},
		{"unknown", MovementType("UNKNOWN"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.value.IsValid(); got != tt.want {
				t.Fatalf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
