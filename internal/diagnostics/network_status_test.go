package diagnostics

import "testing"

func TestNormalizeNetworkAdapter(t *testing.T) {
	t.Parallel()
	code7 := 7
	code99 := 99

	tests := []struct {
		name       string
		in         NetworkAdapter
		wantStatus string
		wantCode   *int
	}{
		{
			name:       "media disconnected from numeric status",
			in:         NetworkAdapter{Name: "Ethernet", Status: "7"},
			wantStatus: NetStatusMediaDisconnected,
			wantCode:   &code7,
		},
		{
			name:       "connected from status code",
			in:         NetworkAdapter{Name: "Ethernet", StatusCode: intPtr(2)},
			wantStatus: NetStatusConnected,
			wantCode:   intPtr(2),
		},
		{
			name:       "unknown code is preserved",
			in:         NetworkAdapter{Name: "Ethernet", Status: "99"},
			wantStatus: "unknown_99",
			wantCode:   &code99,
		},
		{
			name:       "non-numeric status kept when no code",
			in:         NetworkAdapter{Name: "Ethernet", Status: "Up"},
			wantStatus: "Up",
			wantCode:   nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeNetworkAdapter(test.in)
			if got.Status != test.wantStatus {
				t.Fatalf("Status = %q, want %q", got.Status, test.wantStatus)
			}
			if (got.StatusCode == nil) != (test.wantCode == nil) {
				t.Fatalf("StatusCode nil = %v, want nil = %v", got.StatusCode == nil, test.wantCode == nil)
			}
			if got.StatusCode != nil && test.wantCode != nil && *got.StatusCode != *test.wantCode {
				t.Fatalf("StatusCode = %d, want %d", *got.StatusCode, *test.wantCode)
			}
		})
	}
}

func intPtr(value int) *int {
	return &value
}
