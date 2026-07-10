package daemon

import "testing"

func TestIsLoopbackBind(t *testing.T) {
	tests := []struct {
		bind string
		want bool
	}{
		{"", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"0.0.0.0", false},
		{"192.168.1.5", false},
	}
	for _, tt := range tests {
		if got := isLoopbackBind(tt.bind); got != tt.want {
			t.Errorf("isLoopbackBind(%q) = %v, want %v", tt.bind, got, tt.want)
		}
	}
}

func TestBonjourHookNilForLoopback(t *testing.T) {
	if bonjourHook("") != nil {
		t.Error("bonjourHook(\"\") is non-nil, want nil (loopback advertises nothing)")
	}
	if bonjourHook("127.0.0.1") != nil {
		t.Error("bonjourHook(loopback) is non-nil, want nil")
	}
	if bonjourHook("0.0.0.0") == nil {
		t.Error("bonjourHook(0.0.0.0) is nil, want a hook")
	}
}
