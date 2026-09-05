package figlet

import (
	"strings"
	"testing"
)

func TestParseMode(t *testing.T) {
	cases := map[string]Mode{
		"default":  ModeDefault,
		"kern":     ModeKern,
		"KERN":     ModeKern,
		"smush":    ModeSmush,
		"none":     ModeNone,
		"overlap":  ModeOverlap,
		"nonsense": ModeDefault, // anything unrecognized falls back
		"":         ModeDefault,
	}

	for in, want := range cases {
		if got := ParseMode(in); got != want {
			t.Errorf("ParseMode(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestLayoutResolution(t *testing.T) {
	// The five branches of update_figfont_settings(), keyed by the header's
	// old_layout and full_layout fields.
	cases := []struct {
		name     string
		header   string
		wantMode Mode
		wantRule int
	}{
		{
			name:     "old layout -1 means full width",
			header:   "flf2a$ 1 1 6 -1 0 0 0 0",
			wantMode: ModeNone,
		},
		{
			name:     "full layout asks for kerning only",
			header:   "flf2a$ 1 1 6 0 0 0 64 0",
			wantMode: ModeKern,
		},
		{
			name:     "both layouts name rules, so smushing with those rules",
			header:   "flf2a$ 1 1 6 15 0 0 143 0", // 0x8f: smush + rules 1-4
			wantMode: ModeSmush,
			wantRule: 0x0f,
		},
		{
			name:     "full layout asks for universal smushing",
			header:   "flf2a$ 1 1 6 0 0 0 128 0",
			wantMode: ModeSmush,
			wantRule: 0x3f,
		},
		{
			name:     "nothing asked for, so overlap",
			header:   "flf2a$ 1 1 6 0 0 0 0 0",
			wantMode: ModeOverlap,
		},
		{
			name:     "old layout alone names the rules",
			header:   "flf2a$ 1 1 6 15 0 0 0 0",
			wantMode: ModeOverlap,
			wantRule: 15,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := ParseFont(buildFont(tc.header, blockBody))
			if err != nil {
				t.Fatal(err)
			}
			r := NewRenderer(f)
			if r.Mode() != tc.wantMode {
				t.Errorf("mode = %v, want %v", r.Mode(), tc.wantMode)
			}
			if r.Rule() != tc.wantRule {
				t.Errorf("rule = %#x, want %#x", r.Rule(), tc.wantRule)
			}
		})
	}
}

// render lays out a string and returns it as lines of text.
func render(t *testing.T, f *Font, mode Mode, width int, s string) []string {
	t.Helper()

	r := NewRenderer(f)
	r.SetMode(mode)
	if width > 0 {
		r.SetWidth(width)
	}
	for _, ch := range s {
		r.PutChar(ch)
	}
	cv := r.Flush()

	out := make([]string, cv.Height)
	for y := 0; y < cv.Height; y++ {
		var b strings.Builder
		for x := 0; x < cv.Width; x++ {
			b.WriteRune(cv.GetChar(x, y))
		}
		out[y] = b.String()
	}
	return out
}

// barFont is a one-row font whose letters are drawn with vertical bars and
// slashes, padded with a blank column on each side, so every layout mode does
// something visible.
func barFont(t *testing.T, header string) *Font {
	t.Helper()

	body := func(code rune) []string {
		switch code {
		case 'A':
			return []string{" |A| "}
		case 'B':
			return []string{" |B| "}
		case 'C':
			return []string{" \\C/ "}
		case '_':
			return []string{" ___ "}
		}
		return []string{" " + string(code) + " "}
	}

	f, err := ParseFont(buildFont(header, body))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// The expected strings in this file and the next were taken from the C
// program: the synthetic fonts were written out and rendered by toilet 0.3, so
// these tests pin verified output rather than the port's own.
func TestLayoutModes(t *testing.T) {
	// old_layout 15 and full_layout 0x8f: rules 1 to 4, smushing allowed.
	f := barFont(t, "flf2a$ 1 1 10 15 0 0 143 0")

	cases := []struct {
		mode Mode
		want string
	}{
		// Full width keeps the blank columns on both sides of every glyph.
		{ModeNone, " |A|  |B| "},
		// Kerning eats the blanks that meet — including the leading blank of
		// the first glyph, which has nothing to its left to stop it.
		{ModeKern, "|A||B| "},
		// Overlap eats one column more, so the second glyph covers the first
		// glyph's right bar.
		{ModeOverlap, "|A|B| "},
		// Smushing merges the two bars into one under rule 1.
		{ModeSmush, "|A|B| "},
	}

	for _, tc := range cases {
		got := render(t, f, tc.mode, 0, "AB")
		if len(got) != 1 || got[0] != tc.want {
			t.Errorf("mode %v: got %q, want %q", tc.mode, got, tc.want)
		}
	}
}

func TestSmushingUsesTheRules(t *testing.T) {
	f := barFont(t, "flf2a$ 1 1 10 15 0 0 143 0")

	// "AC" puts the right bar of A against the left slash of C. Rule 3 says
	// the slash class beats the bar class, so the slash wins.
	if got := render(t, f, ModeSmush, 0, "AC"); got[0] != "|A\\C/ " {
		t.Errorf("A then C smushed to %q, want %q", got[0], "|A\\C/ ")
	}

	// An underscore against a bar is rule 2: the bar wins.
	if got := render(t, f, ModeSmush, 0, "_A"); got[0] != "__|A| " {
		t.Errorf("_ then A smushed to %q, want %q", got[0], "__|A| ")
	}
}

func TestHardblankHoldsAColumnOpen(t *testing.T) {
	body := func(code rune) []string {
		if code == 'A' {
			return []string{"|$|"}
		}
		return []string{" " + string(code) + " "}
	}
	f, err := ParseFont(buildFont("flf2a$ 1 1 8 15 0 0 143 0", body))
	if err != nil {
		t.Fatal(err)
	}

	// The hardblank sits between two bars and stops them being kerned
	// together, but comes out as a space.
	got := render(t, f, ModeSmush, 0, "AA")
	if got[0] != "| | |" {
		t.Errorf("smushed hardblanks rendered as %q, want %q", got[0], "| | |")
	}
	if strings.ContainsRune(got[0], 0xa0) {
		t.Errorf("output %q still contains a raw hardblank", got[0])
	}
	if strings.ContainsRune(got[0], '$') {
		t.Errorf("output %q still contains the hardblank character", got[0])
	}
}

func TestWrappingAtWidth(t *testing.T) {
	f := barFont(t, "flf2a$ 1 1 10 -1 0 0 0 0") // full width, 5 columns a glyph

	// Three glyphs at five columns each need 15, so a width of 12 wraps after
	// the second.
	got := render(t, f, ModeNone, 12, "ABA")
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2: %q", len(got), got)
	}
	if got[0] != " |A|  |B| " || strings.TrimRight(got[1], " ") != " |A|" {
		t.Errorf("wrapped output = %q", got)
	}
}

func TestNewlineStartsARow(t *testing.T) {
	f := barFont(t, "flf2a$ 1 1 10 -1 0 0 0 0")

	r := NewRenderer(f)
	for _, ch := range "A\nB" {
		r.PutChar(ch)
	}
	cv := r.Flush()

	if cv.Height != 2 {
		t.Fatalf("height = %d, want 2", cv.Height)
	}
	if cv.GetChar(1, 0) != '|' || cv.GetChar(2, 1) != 'B' {
		t.Errorf("newline did not start a second row")
	}
}

func TestUnknownGlyphIsSkipped(t *testing.T) {
	f := barFont(t, "flf2a$ 1 1 10 -1 0 0 0 0")

	// U+2603 is not in the font, so it contributes nothing at all.
	with := render(t, f, ModeNone, 0, "A☃B")
	without := render(t, f, ModeNone, 0, "AB")
	if len(with) != len(without) || with[0] != without[0] {
		t.Errorf("unknown glyph changed the output: %q vs %q", with, without)
	}
}

func TestCarriageReturnIsIgnored(t *testing.T) {
	f := barFont(t, "flf2a$ 1 1 10 -1 0 0 0 0")

	if got, want := render(t, f, ModeNone, 0, "A\rB"), render(t, f, ModeNone, 0, "AB"); got[0] != want[0] {
		t.Errorf("carriage return changed the output: %q vs %q", got, want)
	}
}

func TestFlushResetsTheRenderer(t *testing.T) {
	f := barFont(t, "flf2a$ 1 1 10 -1 0 0 0 0")
	r := NewRenderer(f)

	r.PutChar('A')
	first := r.Flush()
	r.PutChar('B')
	second := r.Flush()

	if first.Width != second.Width || first.Height != second.Height {
		t.Fatalf("second line has a different shape: %dx%d vs %dx%d",
			second.Width, second.Height, first.Width, first.Height)
	}
	if first.GetChar(2, 0) != 'A' || second.GetChar(2, 0) != 'B' {
		t.Errorf("lines bled into each other: %q then %q",
			first.GetChar(2, 0), second.GetChar(2, 0))
	}
	if r.Lines != 2 {
		t.Errorf("Lines = %d, want 2", r.Lines)
	}
}
