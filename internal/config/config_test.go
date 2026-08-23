package config

import "testing"

func TestModeSelection(t *testing.T) {
	tests := []struct {
		env  string // AGENTDECK_MODE
		user string // current user (simulated via detect override)
		want Mode
	}{
		{"", "goat", ModePersonal},
		{"auto", "goat", ModePersonal},
		{"personal", "aiagent", ModePersonal}, // explicit wins
		{"dedicated", "goat", ModeDedicated},  // explicit wins
		{"garbage", "goat", ModePersonal},
	}
	original := detectMode
	for _, tt := range tests {
		t.Run(tt.env+"/"+tt.user, func(t *testing.T) {
			t.Setenv("AGENTDECK_MODE", tt.env)
			detectMode = func() Mode {
				if tt.user == DedicatedUser {
					return ModeDedicated
				}
				return ModePersonal
			}
			c := FromEnv()
			if c.Mode != tt.want {
				t.Errorf("mode = %s, want %s", c.Mode, tt.want)
			}
		})
	}
	detectMode = original
}

func TestDefaults(t *testing.T) {
	c := FromEnv()
	if c.Addr != "0.0.0.0:8444" {
		t.Errorf("addr = %s", c.Addr)
	}
	if !c.TLS {
		t.Error("TLS should default on")
	}
}
