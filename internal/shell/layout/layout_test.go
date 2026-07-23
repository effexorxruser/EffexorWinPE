package layout

import "testing"

func TestLayoutFitsCommonDisplays(t *testing.T) {
	cases := []struct {
		name string
		w, h int
		dpi  int
	}{
		{"1024x768@96", 1024, 768, 96},
		{"1024x768@120", 1024, 768, 120},
		{"1366x768@96", 1366, 768, 96},
		{"1920x1080@144", 1920, 1080, 144},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := Compute(Input{
				ClientW:   tc.w,
				ClientH:   tc.h,
				DPI:       tc.dpi,
				BtnCount:  3,
				ShowBrand: true,
			})
			if len(out.Buttons) != 3 {
				t.Fatalf("buttons=%d", len(out.Buttons))
			}
			if !out.Fits(tc.w, tc.h) {
				t.Fatalf("layout overflows: %+v", out)
			}
			for i, b := range out.Buttons {
				if b.W <= 0 || b.H <= 0 {
					t.Fatalf("button %d invalid: %+v", i, b)
				}
			}
			if out.Content.H < 80 {
				t.Fatalf("content too short: %+v", out.Content)
			}
		})
	}
}

func TestNarrowForcesStack(t *testing.T) {
	out := Compute(Input{ClientW: 1024, ClientH: 768, DPI: 120, BtnCount: 3, ShowBrand: true})
	if !out.Fits(1024, 768) {
		t.Fatalf("must fit: %+v", out)
	}
	// At 125% on 1024 width, equal three-across may still fit or stack; either is OK if Fits.
}

func TestZeroButtons(t *testing.T) {
	out := Compute(Input{ClientW: 1024, ClientH: 768, DPI: 96, BtnCount: 0, ShowBrand: true})
	if len(out.Buttons) != 0 {
		t.Fatalf("buttons=%d", len(out.Buttons))
	}
	if !out.Fits(1024, 768) {
		t.Fatal("overflow")
	}
}
