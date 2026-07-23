package diagnostics

import "testing"

func TestNormalizeWindowsVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   WindowsVersion
		want string
	}{
		{
			name: "windows 10 build 19045 stays windows 10",
			in:   WindowsVersion{RawProductName: "Windows 10 Pro", InstallationType: "Client", Build: "19045.3803"},
			want: "Windows 10 Pro",
		},
		{
			name: "windows 11 build 22000 rewrites legacy product name",
			in:   WindowsVersion{RawProductName: "Windows 10 Pro", InstallationType: "Client", Build: "22000.318"},
			want: "Windows 11 Pro",
		},
		{
			name: "windows 11 build 22621 rewrites legacy product name",
			in:   WindowsVersion{ProductName: "Windows 10 Pro", DisplayVersion: "22H2", InstallationType: "Client", Build: "22621.1848"},
			want: "Windows 11 Pro",
		},
		{
			name: "windows 11 build 26100",
			in:   WindowsVersion{RawProductName: "Windows 10 Home", InstallationType: "Client", Build: "26100.1"},
			want: "Windows 11 Home",
		},
		{
			name: "windows 11 build 26200",
			in:   WindowsVersion{RawProductName: "Windows 10 Pro", InstallationType: "Client", Build: "26200.6584"},
			want: "Windows 11 Pro",
		},
		{
			name: "windows server is not rewritten by client build rules",
			in:   WindowsVersion{RawProductName: "Windows Server 2022 Datacenter", InstallationType: "Server", Build: "22621.1848"},
			want: "Windows Server 2022 Datacenter",
		},
		{
			name: "server product name without installation type",
			in:   WindowsVersion{RawProductName: "Windows Server 2019 Standard", Build: "26100.1"},
			want: "Windows Server 2019 Standard",
		},
		{
			name: "missing build keeps raw product name",
			in:   WindowsVersion{RawProductName: "Windows 10 Pro", InstallationType: "Client"},
			want: "Windows 10 Pro",
		},
		{
			name: "invalid build keeps raw product name",
			in:   WindowsVersion{RawProductName: "Windows 10 Pro", InstallationType: "Client", Build: "not-a-build"},
			want: "Windows 10 Pro",
		},
		{
			name: "already windows 11 product name is unchanged",
			in:   WindowsVersion{RawProductName: "Windows 11 Pro", InstallationType: "Client", Build: "22621.1848"},
			want: "Windows 11 Pro",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeWindowsVersion(test.in)
			if got.ProductName != test.want {
				t.Fatalf("ProductName = %q, want %q", got.ProductName, test.want)
			}
			raw := stringsOr(test.in.RawProductName, test.in.ProductName)
			if got.RawProductName != raw {
				t.Fatalf("RawProductName = %q, want %q", got.RawProductName, raw)
			}
		})
	}
}

func TestParseWindowsBuildNumber(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in     string
		want   uint64
		wantOK bool
	}{
		{in: "22621.1848", want: 22621, wantOK: true},
		{in: "10.0.22621.1848", want: 22621, wantOK: true},
		{in: "19045", want: 19045, wantOK: true},
		{in: "", wantOK: false},
		{in: "abc", wantOK: false},
	}
	for _, test := range tests {
		got, ok := ParseWindowsBuildNumber(test.in)
		if ok != test.wantOK || got != test.want {
			t.Fatalf("ParseWindowsBuildNumber(%q) = (%d, %v), want (%d, %v)", test.in, got, ok, test.want, test.wantOK)
		}
	}
}

func stringsOr(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
