package export

import (
	"strings"
	"testing"
)

type fakeScanner struct{ drives []DriveInfo }

func (f fakeScanner) List() ([]DriveInfo, error) { return f.drives, nil }

func TestCandidateDrivesPreferRemovableExcludeSystem(t *testing.T) {
	scanner := fakeScanner{drives: []DriveInfo{
		{Root: `X:\`, Letter: "X", Label: "WinPE", Kind: DriveFixed, SizeBytes: 1},
		{Root: `C:\`, Letter: "C", Label: "Windows", Kind: DriveFixed, SizeBytes: 500 << 30},
		{Root: `E:\`, Letter: "E", Label: "USB", Kind: DriveRemovable, SizeBytes: 16 << 30},
		{Root: `D:\`, Letter: "D", Label: "DATA", Kind: DriveFixed, SizeBytes: 200 << 30},
	}}
	policy := ExportPolicy{ExcludeRoots: DefaultExcludeRoots([]string{`C:\Windows`})}
	removable, fixed, err := CandidateDrives(scanner, policy)
	if err != nil {
		t.Fatal(err)
	}
	if len(removable) != 1 || removable[0].Letter != "E" {
		t.Fatalf("removable=%+v", removable)
	}
	for _, d := range fixed {
		if d.Letter == "C" || d.Letter == "X" {
			t.Fatalf("fixed contains excluded drive: %+v", fixed)
		}
	}
	if len(fixed) != 1 || fixed[0].Letter != "D" {
		t.Fatalf("fixed=%+v", fixed)
	}
}

func TestFormatDriveIncludesMetadata(t *testing.T) {
	s := FormatDrive(DriveInfo{Root: `E:\`, Label: "BACKUP", SizeBytes: 16 << 30, Kind: DriveRemovable})
	for _, part := range []string{`E:\`, "BACKUP", "GiB", "removable"} {
		if !strings.Contains(s, part) {
			t.Fatalf("format=%q missing %q", s, part)
		}
	}
}
