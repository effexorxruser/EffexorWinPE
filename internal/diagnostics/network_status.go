package diagnostics

import (
	"fmt"
	"strconv"
	"strings"
)

// Stable Win32_NetworkAdapter.NetConnectionStatus names.
const (
	NetStatusDisconnected            = "disconnected"
	NetStatusConnecting              = "connecting"
	NetStatusConnected               = "connected"
	NetStatusDisconnecting           = "disconnecting"
	NetStatusHardwareNotPresent      = "hardware_not_present"
	NetStatusHardwareDisabled        = "hardware_disabled"
	NetStatusHardwareMalfunction     = "hardware_malfunction"
	NetStatusMediaDisconnected       = "media_disconnected"
	NetStatusAuthenticating          = "authenticating"
	NetStatusAuthenticationSucceeded = "authentication_succeeded"
	NetStatusAuthenticationFailed    = "authentication_failed"
	NetStatusInvalidAddress          = "invalid_address"
	NetStatusCredentialsRequired     = "credentials_required"
)

var networkConnectionStatusNames = map[int]string{
	0:  NetStatusDisconnected,
	1:  NetStatusConnecting,
	2:  NetStatusConnected,
	3:  NetStatusDisconnecting,
	4:  NetStatusHardwareNotPresent,
	5:  NetStatusHardwareDisabled,
	6:  NetStatusHardwareMalfunction,
	7:  NetStatusMediaDisconnected,
	8:  NetStatusAuthenticating,
	9:  NetStatusAuthenticationSucceeded,
	10: NetStatusAuthenticationFailed,
	11: NetStatusInvalidAddress,
	12: NetStatusCredentialsRequired,
}

// NormalizeNetworkAdapter derives Status and StatusCode from provider output.
// Unknown numeric codes are preserved as status_code with status "unknown_<code>".
// Non-numeric legacy status strings are kept as-is when no status_code is present.
func NormalizeNetworkAdapter(adapter NetworkAdapter) NetworkAdapter {
	if adapter.StatusCode != nil {
		adapter.Status = NetworkStatusName(*adapter.StatusCode)
		return adapter
	}
	code, ok := parseNetworkStatusCode(adapter.Status)
	if !ok {
		return adapter
	}
	adapter.StatusCode = &code
	adapter.Status = NetworkStatusName(code)
	return adapter
}

// NetworkStatusName maps a Win32_NetworkAdapter.NetConnectionStatus code.
func NetworkStatusName(code int) string {
	if name, ok := networkConnectionStatusNames[code]; ok {
		return name
	}
	return fmt.Sprintf("unknown_%d", code)
}

func parseNetworkStatusCode(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}
