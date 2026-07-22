package collector

import "testing"

func TestParseRegistryValue(t *testing.T) {
	output := `
HKEY_LOCAL_MACHINE\ANPOFFLINE\Microsoft\Windows NT\CurrentVersion
    ProductName    REG_SZ    Windows 11 Pro
`
	if got := parseRegistryValue(output); got != "Windows 11 Pro" {
		t.Fatalf("parseRegistryValue() = %q, want %q", got, "Windows 11 Pro")
	}
}

func TestParseRegistryInteger(t *testing.T) {
	if got := parseRegistryInteger("0x1234"); got != "4660" {
		t.Fatalf("parseRegistryInteger() = %q, want 4660", got)
	}
}
