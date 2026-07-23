package winui

import "testing"

func TestShouldExitKioskOnKeyMessage(t *testing.T) {
	cases := []struct {
		name   string
		msg    uint32
		wParam uintptr
		kiosk  bool
		want   bool
	}{
		{"esc in kiosk keydown", keyMsgKeyDown, keyVKEscape, true, true},
		{"esc in kiosk syskeydown", keyMsgSysKeyDown, keyVKEscape, true, true},
		{"esc windowed ignored", keyMsgKeyDown, keyVKEscape, false, false},
		{"other key ignored", keyMsgKeyDown, 0x41, true, false},
		{"mousemove ignored", 0x0200, keyVKEscape, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ShouldExitKioskOnKeyMessage(tc.msg, tc.wParam, tc.kiosk)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
