package collector

import (
	"testing"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

func TestFilterOfflineWindowsInstallationsExcludesRuntimeWinPE(t *testing.T) {
	t.Parallel()
	got := filterOfflineWindowsInstallations(candidateInstallations(`D:\`, `X:\`), `X:\Windows`, true, nil)
	if len(got) != 1 || got[0].Root != `D:\` {
		t.Fatalf("got %+v, want only D:\\", got)
	}
}

func TestFilterOfflineWindowsInstallationsKeepsTwoRealInstalls(t *testing.T) {
	t.Parallel()
	got := filterOfflineWindowsInstallations(candidateInstallations(`C:\`, `D:\`, `X:\`), `X:\WINDOWS`, true, nil)
	if len(got) != 2 {
		t.Fatalf("got %d installations, want 2", len(got))
	}
	if got[0].Root != `C:\` || got[1].Root != `D:\` {
		t.Fatalf("got roots %q and %q", got[0].Root, got[1].Root)
	}
}

func TestFilterOfflineWindowsInstallationsOnlyRuntime(t *testing.T) {
	t.Parallel()
	got := filterOfflineWindowsInstallations(candidateInstallations(`X:\`), `X:\Windows`, true, nil)
	if len(got) != 0 {
		t.Fatalf("got %d installations, want 0", len(got))
	}
}

func TestFilterOfflineWindowsInstallationsRuntimeNotHardCodedToX(t *testing.T) {
	t.Parallel()
	got := filterOfflineWindowsInstallations(candidateInstallations(`D:\`, `W:\`), `W:\Windows`, true, nil)
	if len(got) != 1 || got[0].Root != `D:\` {
		t.Fatalf("got %+v, want only D:\\ when runtime root is W:\\", got)
	}
}

func TestFilterOfflineWindowsInstallationsUsesWinPERuntimeMarkers(t *testing.T) {
	t.Parallel()
	exists := func(path string) bool {
		return path == `Y:\Windows\System32\winpeshl.exe`
	}
	got := filterOfflineWindowsInstallations(candidateInstallations(`D:\`, `Y:\`), ``, true, exists)
	if len(got) != 1 || got[0].Root != `D:\` {
		t.Fatalf("got %+v, want only D:\\ after WinPE marker exclusion", got)
	}
}

func TestFilterOfflineWindowsInstallationsKeepsWinPEMediaOutsideWinPE(t *testing.T) {
	t.Parallel()
	exists := func(path string) bool {
		return path == `Y:\Windows\System32\winpeshl.exe`
	}
	// Marker-based exclusion applies only while the live environment is WinPE.
	got := filterOfflineWindowsInstallations(candidateInstallations(`D:\`, `Y:\`), `C:\Windows`, false, exists)
	if len(got) != 2 {
		t.Fatalf("got %d installations, want 2 when runtime is not WinPE", len(got))
	}
}

func TestCurrentEnvironmentIsWinPEFalseOutsideWindowsBuild(t *testing.T) {
	t.Parallel()
	if currentEnvironmentIsWinPE() {
		t.Fatal("currentEnvironmentIsWinPE() = true on non-Windows test host")
	}
}

func TestNormalizeWindowsInstallRoot(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		`X:\Windows`:  `X:`,
		`X:\WINDOWS\`: `X:`,
		`D:\`:         `D:`,
		`d:\windows`:  `d:`,
		`C:/Windows`:  `C:`,
		``:            ``,
	}
	for input, want := range tests {
		if got := normalizeWindowsInstallRoot(input); got != want {
			t.Fatalf("normalizeWindowsInstallRoot(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLooksLikeWinPERoot(t *testing.T) {
	t.Parallel()
	exists := func(path string) bool {
		return path == `X:\Windows\System32\winpeshl.exe`
	}
	if !looksLikeWinPERoot(`X:\`, exists) {
		t.Fatal("expected WinPE markers under X:\\")
	}
	if looksLikeWinPERoot(`D:\`, exists) {
		t.Fatal("ordinary Windows root must not look like WinPE without markers")
	}
}

func candidateInstallations(roots ...string) []diagnostics.Installation {
	installations := make([]diagnostics.Installation, 0, len(roots))
	for _, root := range roots {
		installations = append(installations, diagnostics.Installation{Root: root})
	}
	return installations
}
