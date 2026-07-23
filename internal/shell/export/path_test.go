package export

import (
	"strings"
	"testing"
)

func TestEvaluateExportPathNestedCannotBypass(t *testing.T) {
	scanner := fakeScanner{drives: []DriveInfo{
		{Root: `X:\`, Letter: "X", Kind: DriveFixed},
		{Root: `C:\`, Letter: "C", Label: "Windows", Kind: DriveFixed, SizeBytes: 500 << 30},
		{Root: `D:\`, Letter: "D", Label: "DATA", Kind: DriveFixed, SizeBytes: 200 << 30},
		{Root: `E:\`, Letter: "E", Label: "USB", Kind: DriveRemovable, SizeBytes: 16 << 30},
		{Root: `F:\`, Letter: "F", Label: "CD", Kind: DriveOther},
	}}
	policy := ExportPolicy{ExcludeRoots: DefaultExcludeRoots([]string{`C:\Windows`})}

	cases := []struct {
		path string
		want PathDecision
	}{
		{`E:\reports\session`, PathAllowRemovable},
		{`E:/nested/dir`, PathAllowRemovable},
		{`D:\EffexorWinPE-reports`, PathRequireFixedConfirm},
		{`C:\Users\Public\Documents`, PathRejectExcluded},
		{`C:\temp\out`, PathRejectExcluded},
		{`X:\EffexorWinPE\reports`, PathRejectExcluded},
		{`F:\iso`, PathRejectUnknown},
		{`Z:\no-such`, PathRejectUnknown},
		{`\\server\share`, PathRejectUnknown},
		{"", PathRejectUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			eval, err := EvaluateExportPath(tc.path, scanner, policy)
			if err != nil {
				t.Fatal(err)
			}
			if eval.Decision != tc.want {
				t.Fatalf("path=%q decision=%v want=%v eval=%+v", tc.path, eval.Decision, tc.want, eval)
			}
			if tc.path != "" && VolumeRoot(tc.path) != "" && eval.VolumeRoot == "" {
				t.Fatal("expected volume root")
			}
		})
	}
}

func TestVolumeRootFromNestedPath(t *testing.T) {
	if got := VolumeRoot(`d:\client\windows\system32`); !strings.EqualFold(got, `D:\`) {
		t.Fatalf("got %q", got)
	}
	if got := VolumeRoot(`X:/Windows`); got != `X:\` {
		t.Fatalf("got %q", got)
	}
}
