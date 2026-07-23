package layout

// Rect is an integer rectangle in client coordinates.
type Rect struct {
	X, Y, W, H int
}

// Input describes the available client area and which action buttons are visible.
type Input struct {
	ClientW   int
	ClientH   int
	DPI       int // 96 = 100%, 120 = 125%, 144 = 150%
	NavCount  int
	BtnCount  int // number of visible action buttons (0..3)
	ShowBrand bool
}

// Output is a full shell layout.
type Output struct {
	Brand   Rect
	Nav     Rect
	Content Rect
	Buttons []Rect
	// ButtonsStacked is true when buttons wrap to a second row or column.
	ButtonsStacked bool
}

// Compute returns a layout that keeps every control inside the client rect.
func Compute(in Input) Output {
	if in.DPI <= 0 {
		in.DPI = 96
	}
	if in.ClientW < 1 {
		in.ClientW = 1
	}
	if in.ClientH < 1 {
		in.ClientH = 1
	}
	if in.BtnCount < 0 {
		in.BtnCount = 0
	}
	if in.BtnCount > 3 {
		in.BtnCount = 3
	}
	scale := float64(in.DPI) / 96.0
	pad := int(16 * scale)
	if pad < 8 {
		pad = 8
	}
	gap := int(12 * scale)
	if gap < 6 {
		gap = 6
	}
	navW := int(240 * scale)
	if navW > in.ClientW/3 {
		navW = in.ClientW / 3
	}
	if navW < int(160*scale) {
		navW = int(160 * scale)
	}
	if navW+2*pad > in.ClientW {
		navW = in.ClientW - 2*pad
		if navW < 80 {
			navW = 80
		}
	}
	brandH := 0
	if in.ShowBrand {
		brandH = int(44 * scale)
	}
	btnH := int(40 * scale)
	if btnH < 32 {
		btnH = 32
	}

	out := Output{}
	contentX := pad + navW + pad
	contentW := in.ClientW - contentX - pad
	if contentW < 120 {
		// Collapse nav width to keep content usable.
		navW = int(140 * scale)
		contentX = pad + navW + pad
		contentW = in.ClientW - contentX - pad
		if contentW < 80 {
			contentW = 80
			contentX = in.ClientW - pad - contentW
			if contentX < pad {
				contentX = pad
			}
		}
	}

	brandY := pad
	if in.ShowBrand {
		out.Brand = Rect{X: pad, Y: brandY, W: navW, H: brandH}
	}
	navY := pad + brandH
	if in.ShowBrand {
		navY += gap / 2
	}
	out.Nav = Rect{X: pad, Y: navY, W: navW, H: in.ClientH - navY - pad}
	if out.Nav.H < 40 {
		out.Nav.H = 40
	}

	// Decide button arrangement from available content width.
	buttons := make([]Rect, 0, in.BtnCount)
	stacked := false
	btnAreaTop := 0
	if in.BtnCount > 0 {
		minBtnW := int(140 * scale)
		idealBtnW := (contentW - gap*(in.BtnCount-1)) / in.BtnCount
		if idealBtnW >= minBtnW {
			// Single row, equal widths filling content width.
			btnY := in.ClientH - pad - btnH
			x := contentX
			bw := idealBtnW
			for i := 0; i < in.BtnCount; i++ {
				w := bw
				if i == in.BtnCount-1 {
					// Absorb rounding remainder.
					w = contentX + contentW - x
				}
				buttons = append(buttons, Rect{X: x, Y: btnY, W: w, H: btnH})
				x += w + gap
			}
			btnAreaTop = btnY
		} else if in.BtnCount > 1 {
			// Wrap to two rows (or vertical stack when still too narrow).
			stacked = true
			cols := 2
			if contentW < minBtnW*2+gap {
				cols = 1
			}
			rows := (in.BtnCount + cols - 1) / cols
			btnAreaH := rows*btnH + (rows-1)*gap
			btnAreaTop = in.ClientH - pad - btnAreaH
			colW := (contentW - gap*(cols-1)) / cols
			if colW < minBtnW && cols > 1 {
				cols = 1
				rows = in.BtnCount
				btnAreaH = rows*btnH + (rows-1)*gap
				btnAreaTop = in.ClientH - pad - btnAreaH
				colW = contentW
			}
			for i := 0; i < in.BtnCount; i++ {
				row := i / cols
				col := i % cols
				x := contentX + col*(colW+gap)
				y := btnAreaTop + row*(btnH+gap)
				w := colW
				if col == cols-1 {
					w = contentX + contentW - x
				}
				buttons = append(buttons, Rect{X: x, Y: y, W: w, H: btnH})
			}
		} else {
			btnY := in.ClientH - pad - btnH
			buttons = append(buttons, Rect{X: contentX, Y: btnY, W: contentW, H: btnH})
			btnAreaTop = btnY
		}
	} else {
		btnAreaTop = in.ClientH - pad
	}

	contentY := pad + brandH
	if in.ShowBrand {
		contentY = pad + brandH + gap/2
	}
	contentH := btnAreaTop - gap - contentY
	if contentH < 80 {
		contentH = 80
	}
	out.Content = Rect{X: contentX, Y: contentY, W: contentW, H: contentH}
	out.Buttons = buttons
	out.ButtonsStacked = stacked

	// Clamp any overflow caused by extreme sizes.
	clamp := func(r *Rect) {
		if r.X < 0 {
			r.X = 0
		}
		if r.Y < 0 {
			r.Y = 0
		}
		if r.X+r.W > in.ClientW {
			r.W = in.ClientW - r.X
		}
		if r.Y+r.H > in.ClientH {
			r.H = in.ClientH - r.Y
		}
		if r.W < 0 {
			r.W = 0
		}
		if r.H < 0 {
			r.H = 0
		}
	}
	clamp(&out.Brand)
	clamp(&out.Nav)
	clamp(&out.Content)
	for i := range out.Buttons {
		clamp(&out.Buttons[i])
	}
	return out
}

// Fits reports whether every rect is inside the client area and non-overlapping
// enough for smoke assertions (buttons fully visible).
func (o Output) Fits(clientW, clientH int) bool {
	inside := func(r Rect) bool {
		if r.W <= 0 || r.H <= 0 {
			return false
		}
		return r.X >= 0 && r.Y >= 0 && r.X+r.W <= clientW && r.Y+r.H <= clientH
	}
	if o.Nav.W > 0 && !inside(o.Nav) {
		return false
	}
	if o.Content.W > 0 && !inside(o.Content) {
		return false
	}
	for _, b := range o.Buttons {
		if !inside(b) {
			return false
		}
	}
	return true
}
