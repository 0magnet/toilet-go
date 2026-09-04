package figlet

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// buildFont assembles a FIGfont in memory. The body function returns the rows
// of the glyph for a code point; each row is written with the end-of-line
// marker appended, doubled on the last row as real fonts do.
func buildFont(hdr string, body func(rune) []string) []byte {
	var b bytes.Buffer

	// The declared height and comment count govern the shape of the file.
	height, comments := 1, 0
	fields := strings.Fields(hdr)
	if len(fields) > 1 {
		height, _ = strconv.Atoi(fields[1])
	}
	if len(fields) > 5 {
		comments, _ = strconv.Atoi(fields[5])
	}

	b.WriteString(hdr)
	b.WriteByte('\n')
	for i := 0; i < comments; i++ {
		fmt.Fprintf(&b, "comment line %d\n", i)
	}

	writeGlyph := func(code rune) {
		rows := body(code)
		for i, r := range rows {
			b.WriteString(r)
			if i == len(rows)-1 {
				b.WriteString("@@\n")
			} else {
				b.WriteString("@\n")
			}
		}
		for i := len(rows); i < height; i++ {
			b.WriteString("@\n")
		}
	}

	for c := rune(32); c < 127; c++ {
		writeGlyph(c)
	}
	for _, c := range deutsch {
		writeGlyph(c)
	}

	return b.Bytes()
}

// blockBody gives every glyph the same single row: the character itself
// surrounded by two spaces, so kerning and smushing have something to eat.
func blockBody(code rune) []string { return []string{" " + string(code) + " "} }

// header collects the scalar fields of a parsed header, so a test can compare
// them in one go.
type header struct {
	Hardblank                    rune
	Height, Baseline, MaxLength  int
	OldLayout, FullLayout        int
	PrintDirection, CodetagCount int
}

func TestParseHeader(t *testing.T) {
	cases := []struct {
		name    string
		header  string
		want    header
		wantErr bool
	}{
		{
			name:   "figlet standard",
			header: "flf2a$ 6 5 16 15 15 0 24463 229",
			want: header{Hardblank: '$', Height: 6, Baseline: 5, MaxLength: 16,
				OldLayout: 15, FullLayout: 24463, PrintDirection: 0,
				CodetagCount: 229},
		},
		{
			name:   "toilet, DEL hardblank",
			header: "tlf2a\x7f 3 3 8 -1 22 0 0 0",
			want: header{Hardblank: 0x7f, Height: 3, Baseline: 3, MaxLength: 8,
				OldLayout: -1},
		},
		{
			name:   "six fields is enough",
			header: "flf2a$ 4 3 10 0 2",
			want:   header{Hardblank: '$', Height: 4, Baseline: 3, MaxLength: 10},
		},
		{
			name:   "print direction right to left",
			header: "flf2a$ 6 5 76 15 14 1 16271 39",
			want: header{Hardblank: '$', Height: 6, Baseline: 5, MaxLength: 76,
				OldLayout: 15, FullLayout: 16271, PrintDirection: 1,
				CodetagCount: 39},
		},
		{
			name:   "multi-byte hardblank",
			header: "tlf2a§ 2 1 4 0 1 0 0 0",
			want:   header{Hardblank: '§', Height: 2, Baseline: 1, MaxLength: 4},
		},
		{
			name:   "hex old layout, scanned with %i",
			header: "flf2a$ 2 1 4 0x0f 1 0 0 0",
			want: header{Hardblank: '$', Height: 2, Baseline: 1, MaxLength: 4,
				OldLayout: 15},
		},

		{name: "no signature", header: "nope 1 1 1 0 0", wantErr: true},
		{name: "wrong signature", header: "flf2b$ 1 1 1 0 0", wantErr: true},
		{name: "five fields", header: "flf2a$ 4 3 10 0", wantErr: true},
		{name: "old layout too small", header: "flf2a$ 4 3 10 -2 0", wantErr: true},
		{name: "old layout too large", header: "flf2a$ 4 3 10 64 0", wantErr: true},
		{name: "full layout too large", header: "flf2a$ 4 3 10 0 0 0 32768 0", wantErr: true},
		{
			// full_layout asks for smushing with no rules while old_layout
			// still names some: libcaca rejects the contradiction.
			name:    "contradictory layout",
			header:  "flf2a$ 4 3 10 15 0 0 128 0",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := buildFont(tc.header, blockBody)
			f, err := ParseFont(data)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got a font")
				}
				if !errors.Is(err, ErrBadFont) {
					t.Fatalf("error %v does not wrap ErrBadFont", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := header{Hardblank: f.Hardblank, Height: f.Height,
				Baseline: f.Baseline, MaxLength: f.MaxLength,
				OldLayout: f.OldLayout, FullLayout: f.FullLayout,
				PrintDirection: f.PrintDirection, CodetagCount: f.CodetagCount}
			if got != tc.want {
				t.Errorf("header fields:\n got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

func TestCommentLinesAreSkipped(t *testing.T) {
	// A comment that looks like a header must not be mistaken for one.
	data := buildFont("flf2a$ 1 1 6 0 3", blockBody)
	f, err := ParseFont(data)
	if err != nil {
		t.Fatal(err)
	}
	if w, ok := f.GlyphWidth('A'); !ok || w != 3 {
		t.Errorf("glyph A width = %d, %v; want 3, true", w, ok)
	}
}

func TestMandatoryGlyphs(t *testing.T) {
	f, err := ParseFont(buildFont("flf2a$ 1 1 6 0 0", blockBody))
	if err != nil {
		t.Fatal(err)
	}

	if f.Glyphs() != extGlyphs {
		t.Fatalf("Glyphs() = %d, want %d", f.Glyphs(), extGlyphs)
	}

	// The 95 printable ASCII glyphs come first, in code point order, then the
	// seven German ones in the order the specification fixes.
	for c := rune(32); c < 127; c++ {
		if _, ok := f.GlyphWidth(c); !ok {
			t.Errorf("missing mandatory glyph %q", c)
		}
	}
	for _, c := range deutsch {
		if _, ok := f.GlyphWidth(c); !ok {
			t.Errorf("missing mandatory glyph U+%04X", c)
		}
	}
	if _, ok := f.GlyphWidth('é'); ok {
		t.Error("é is not in the font but was found")
	}
}

func TestTooFewGlyphs(t *testing.T) {
	full := buildFont("flf2a$ 1 1 6 0 0", blockBody)
	lines := bytes.SplitAfter(full, []byte("\n"))
	// Header plus one row per glyph; drop the last glyph.
	short := bytes.Join(lines[:len(lines)-2], nil)

	if _, err := ParseFont(short); err == nil {
		t.Fatal("expected an error for a font with too few glyphs")
	}
}

func TestGlyphWidthAndHardblanks(t *testing.T) {
	// A three-row glyph whose rows have different lengths, plus hardblanks.
	body := func(code rune) []string {
		if code != 'A' {
			return []string{"x", "x", "x"}
		}
		return []string{"$AA$", "AA", "$$$"}
	}
	f, err := ParseFont(buildFont("flf2a$ 3 2 8 0 0", body))
	if err != nil {
		t.Fatal(err)
	}

	// The width is fixed by the first row that has anything before the marker,
	// which is the top row here: four columns.
	w, ok := f.GlyphWidth('A')
	if !ok || w != 4 {
		t.Fatalf("GlyphWidth('A') = %d, %v; want 4, true", w, ok)
	}

	// Hardblanks are held as U+00A0 inside the font canvas.
	c := f.index('A')
	if got := f.cv.GetChar(0, c*f.Height); got != 0xa0 {
		t.Errorf("hardblank cell = %q, want U+00A0", got)
	}
	if got := f.cv.GetChar(1, c*f.Height); got != 'A' {
		t.Errorf("cell 1 = %q, want 'A'", got)
	}
	// The end-of-line markers are gone.
	if got := f.cv.GetChar(4, c*f.Height); got != ' ' {
		t.Errorf("marker cell = %q, want a space", got)
	}
}

func TestCodeTags(t *testing.T) {
	base := buildFont("flf2a$ 1 1 6 0 0", blockBody)

	var b bytes.Buffer
	b.Write(base)
	b.WriteString("233\n dec @@\n")     // decimal tag
	b.WriteString("0x2665\n hex @@\n")  // hexadecimal tag
	b.WriteString("\n")                 // blank line, as in jacky.flf
	b.WriteString("-1\n skip @@\n")     // negative index, as in ivrit.flf
	b.WriteString("0x263A\n last @@\n") // another hexadecimal tag

	f, err := ParseFont(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []rune{233, 0x2665, 0x263a} {
		if _, ok := f.GlyphWidth(want); !ok {
			t.Errorf("tagged glyph U+%04X missing", want)
		}
	}

	// The blank line and the negative index each still take a slot, which is
	// libcaca's behavior and which shifts every later glyph.
	if f.Glyphs() != extGlyphs+5 {
		t.Errorf("Glyphs() = %d, want %d", f.Glyphs(), extGlyphs+5)
	}
}

func TestBadCodeTag(t *testing.T) {
	var b bytes.Buffer
	b.Write(buildFont("flf2a$ 1 1 6 0 0", blockBody))
	b.WriteString("nonsense\n x @@\n")

	if _, err := ParseFont(b.Bytes()); err == nil {
		t.Fatal("expected an error for a non-numeric code tag")
	}
}

func TestCompressedFonts(t *testing.T) {
	plain := buildFont("flf2a$ 1 1 6 0 0", blockBody)

	t.Run("gzip", func(t *testing.T) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		if _, err := zw.Write(plain); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		mustParseSame(t, buf.Bytes(), plain)
	})

	t.Run("zip", func(t *testing.T) {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		w, err := zw.Create("font.flf")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(plain); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		mustParseSame(t, buf.Bytes(), plain)
	})
}

// mustParseSame checks that a packed font parses to the same thing as the
// plain one it was made from.
func mustParseSame(t *testing.T, packed, plain []byte) {
	t.Helper()

	a, err := ParseFont(packed)
	if err != nil {
		t.Fatalf("packed font: %v", err)
	}
	b, err := ParseFont(plain)
	if err != nil {
		t.Fatalf("plain font: %v", err)
	}

	if a.Glyphs() != b.Glyphs() || a.Height != b.Height {
		t.Fatalf("packed font differs: %d glyphs of height %d, want %d of %d",
			a.Glyphs(), a.Height, b.Glyphs(), b.Height)
	}
	if !bytes.Equal([]byte(string(a.cv.Chars)), []byte(string(b.cv.Chars))) {
		t.Error("packed and plain fonts have different glyph data")
	}
}

func TestLineReader(t *testing.T) {
	r := &lineReader{data: []byte("one\ntwo\nthree")}

	for _, want := range []string{"one\n", "two\n", "three"} {
		got, ok := r.gets(2048)
		if !ok || string(got) != want {
			t.Fatalf("gets = %q, %v; want %q, true", got, ok, want)
		}
	}
	if _, ok := r.gets(2048); ok {
		t.Error("gets past the end returned a line")
	}

	// A line longer than the buffer is cut, as caca_file_gets() cuts it.
	r = &lineReader{data: []byte("abcdefgh\n")}
	got, _ := r.gets(4)
	if string(got) != "abc" {
		t.Errorf("truncated line = %q, want %q", got, "abc")
	}
}

func TestScannerNumber(t *testing.T) {
	cases := []struct {
		in       string
		prefixed bool
		want     int
		ok       bool
	}{
		{"42", false, 42, true},
		{" -7", false, -7, true},
		{"+5", false, 5, true},
		{"0x1f", true, 31, true},
		{"017", true, 15, true},
		{"017", false, 17, true},
		{"", false, 0, false},
		{"abc", false, 0, false},
	}

	for _, tc := range cases {
		s := &scanner{b: []byte(tc.in)}
		got, ok := s.number(tc.prefixed)
		if got != tc.want || ok != tc.ok {
			t.Errorf("number(%q, %v) = %d, %v; want %d, %v",
				tc.in, tc.prefixed, got, ok, tc.want, tc.ok)
		}
	}
}

func TestLoadFontMissing(t *testing.T) {
	if _, err := LoadFont("/nonexistent/no-such-font"); err == nil {
		t.Fatal("expected an error for a missing font")
	} else if !strings.Contains(err.Error(), "no-such-font") {
		t.Errorf("error %q does not name the font", err)
	}
}
