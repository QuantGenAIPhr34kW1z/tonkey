package validator

import "testing"

func TestValidateTONAddress(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{"empty address", "", true},
		{"valid EQ address", "EQDx1Vg0K0jMKS2N-W5F0NzZPLGp0tPBtQmILqFmXOL_vXqp", false},
		{"valid UQ address", "UQDx1Vg0K0jMKS2N-W5F0NzZPLGp0tPBtQmILqFmXOL_vXqp", false},
		{"test address", "EQDemoAddr", false},
		{"test address", "EQTestAddr", false},
		{"invalid prefix", "AB1234567890", true},
		{"too short", "EQ123", true},
		{"invalid characters", "EQ@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@", true},
		{"whitespace preserved", "  EQTestAddr  ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTONAddress(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTONAddress(%q) error = %v, wantErr %v", tt.addr, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAmount(t *testing.T) {
	tests := []struct {
		name    string
		amount  int64
		wantErr bool
	}{
		{"positive amount", 1000, false},
		{"zero amount", 0, true},
		{"negative amount", -100, true},
		{"large amount", 1_000_000_000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAmount(tt.amount)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAmount(%d) error = %v, wantErr %v", tt.amount, err, tt.wantErr)
			}
		})
	}
}
