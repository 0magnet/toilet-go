package canvas

import (
	"testing"

	"github.com/0magnet/img2txt-go/caca"
)

func TestDecodeUTF8(t *testing.T) {
	cases := []struct {
		in    string
		want  rune
		bytes int
	}{
		{"A", 'A', 1},
		{"é", 'é', 2},
		{"☃", '☃', 3},
		{"\U0001F600", 0x1f600, 4},
		{"", 0, 0},
	}

	for _, tc := range cases {
		got, n := DecodeUTF8([]byte(tc.in))
		if got != tc.want || n != tc.bytes {
			t.Errorf("DecodeUTF8(%q) = %q, %d; want %q, %d",
				tc.in, got, n, tc.want, tc.bytes)
		}
	}

	// The decoder is libcaca's, which trusts the lead byte's length and does
	// not validate what follows. 0xFF claims five continuation bytes, and the
	// six bytes it then reads accumulate into a value well past U+10FFFF.
	got, n := DecodeUTF8([]byte("\xff\xfe invalid"))
	if n != 6 {
		t.Fatalf("length = %d, want 6", n)
	}
	if got != 0x3c7e8b76 {
		t.Errorf("decoded %#x, want %#x", got, 0x3c7e8b76)
	}

	// A truncated sequence, or one with a zero byte in it, decodes to nothing.
	if _, n := DecodeUTF8([]byte("\xff\xfe")); n != 0 {
		t.Errorf("truncated sequence gave a length of %d, want 0", n)
	}
	if _, n := DecodeUTF8([]byte{0xc3, 0x00}); n != 0 {
		t.Errorf("sequence with a NUL gave a length of %d, want 0", n)
	}
}

func TestImportGrowsToFit(t *testing.T) {
	cv := New(0, 0)
	cv.ImportUTF8([]byte("hello\nworld!\n"))

	// The trailing newline grows the canvas by a row of its own: libcaca runs
	// its grow step for every input byte, control characters included.
	if cv.Width != 6 || cv.Height != 3 {
		t.Fatalf("size = %dx%d, want 6x3", cv.Width, cv.Height)
	}
	if got := rows(cv); got[0] != "hello " || got[1] != "world!" || got[2] != "      " {
		t.Errorf("imported %q", got)
	}
}

func TestImportControlCharacters(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"carriage return rewinds", "abc\rX", []string{"Xbc"}},
		{"tab moves to the next multiple of eight", "a\tb", []string{"a       b"}},
		{"backspace steps back", "abc\bX", []string{"abX"}},
		{"newline starts a row", "ab\ncd", []string{"ab", "cd"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cv := New(0, 0)
			cv.ImportUTF8([]byte(tc.in))
			got := rows(cv)
			if len(got) != len(tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("row %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestImportFormFeedHomesTheCursor(t *testing.T) {
	// A form feed followed by a newline starts a new frame. Only one frame is
	// kept here, and since caca_create_frame() copies the current one the
	// visible effect is just that the cursor goes home.
	cv := New(0, 0)
	cv.ImportUTF8([]byte("ab\f\ncd"))

	if got := rows(cv); len(got) != 1 || got[0] != "cd" {
		t.Errorf("imported %q, want [cd]", got)
	}
}

func TestImportSGR(t *testing.T) {
	cv := New(0, 0)
	cv.ImportUTF8([]byte("\033[31mR\033[0mD"))

	if got := caca.AttrToANSIFg(cv.GetAttr(0, 0)); got != caca.Red {
		t.Errorf("red cell foreground = %d, want %d", got, caca.Red)
	}
	if got := caca.AttrToANSIFg(cv.GetAttr(1, 0)); got != caca.Default {
		t.Errorf("reset cell foreground = %d, want %d", got, caca.Default)
	}

	// Bold brightens one of the low eight colors.
	cv2 := New(0, 0)
	cv2.ImportUTF8([]byte("\033[1;34mB"))
	if got := caca.AttrToANSIFg(cv2.GetAttr(0, 0)); got != caca.LightBlue {
		t.Errorf("bold blue = %d, want %d", got, caca.LightBlue)
	}

	// Reverse video swaps the two colors.
	cv3 := New(0, 0)
	cv3.ImportUTF8([]byte("\033[31;42;7mX"))
	if fg := caca.AttrToANSIFg(cv3.GetAttr(0, 0)); fg != caca.Green {
		t.Errorf("reversed foreground = %d, want %d", fg, caca.Green)
	}
	if bg := caca.AttrToANSIBg(cv3.GetAttr(0, 0)); bg != caca.Red {
		t.Errorf("reversed background = %d, want %d", bg, caca.Red)
	}

	// Concealed text turns both colors transparent.
	cv4 := New(0, 0)
	cv4.ImportUTF8([]byte("\033[8mX"))
	if fg := caca.AttrToANSIFg(cv4.GetAttr(0, 0)); fg != caca.Transparent {
		t.Errorf("concealed foreground = %d, want %d", fg, caca.Transparent)
	}
}

func TestImportCursorMovement(t *testing.T) {
	cv := New(0, 0)
	cv.ImportUTF8([]byte("a\033[4Cb\033[2Dc"))

	// Right four from column one puts b at column five; left two then puts c
	// in the blank at column four.
	if got := rows(cv); got[0] != "a   cb" {
		t.Errorf("imported %q, want %q", got[0], "a   cb")
	}
}

func TestImportSkipsOSCAndPrivateSequences(t *testing.T) {
	cv := New(0, 0)
	cv.ImportUTF8([]byte("\033]0;a title\aX"))
	if got := rows(cv); len(got) != 1 || got[0] != "X" {
		t.Errorf("OSC left %q behind, want [X]", got)
	}

	cv2 := New(0, 0)
	cv2.ImportUTF8([]byte("\033[?25lY"))
	if got := rows(cv2); len(got) != 1 || got[0] != "Y" {
		t.Errorf("private sequence left %q behind, want [Y]", got)
	}
}

func TestImportTruncatedEscapeStops(t *testing.T) {
	cv := New(0, 0)
	cv.ImportUTF8([]byte("ab\033["))

	if got := rows(cv); len(got) != 1 || got[0] != "ab" {
		t.Errorf("imported %q, want [ab]", got)
	}
}

func TestImportInvalidUTF8IsLatin1(t *testing.T) {
	// A byte the decoder cannot make sense of is taken as latin1. 0x80 is a
	// lone continuation byte, so it becomes U+0080.
	cv := New(0, 0)
	cv.ImportUTF8([]byte{'a', 0x80, 'b'})

	if cv.GetChar(1, 0) != 0x80 {
		t.Errorf("cell 1 = %#x, want 0x80", cv.GetChar(1, 0))
	}
}

func TestImportFullwidthTakesTwoCells(t *testing.T) {
	cv := New(0, 0)
	cv.ImportUTF8([]byte("日本"))

	if cv.Width != 4 {
		t.Fatalf("width = %d, want 4", cv.Width)
	}
	if uint32(cv.GetChar(1, 0)) != caca.MagicFullwidth ||
		uint32(cv.GetChar(3, 0)) != caca.MagicFullwidth {
		t.Error("fullwidth glyphs did not claim their second cell")
	}
}

func TestImportLeavesTheCursor(t *testing.T) {
	cv := New(0, 0)
	cv.ImportUTF8([]byte("abc"))

	if cv.X != 3 || cv.Y != 0 {
		t.Errorf("cursor = %d,%d; want 3,0", cv.X, cv.Y)
	}
}

func TestExportCacaCarriesTheFrameInfo(t *testing.T) {
	cv := fill("ab")
	cv.X, cv.Y = 1, 2
	cv.HandleX, cv.HandleY = 3, 4

	data, ok := cv.Export("caca")
	if !ok {
		t.Fatal("caca export failed")
	}
	if string(data[:4]) != "\xCA\xCACV" {
		t.Fatalf("bad magic %q", data[:4])
	}

	u32 := func(off int) uint32 {
		return uint32(data[off])<<24 | uint32(data[off+1])<<16 |
			uint32(data[off+2])<<8 | uint32(data[off+3])
	}
	// The frame info follows the twenty byte canvas header: width, height,
	// duration, attribute, then the cursor and handle.
	for _, tc := range []struct {
		off  int
		want uint32
		name string
	}{
		{20, 2, "width"},
		{24, 1, "height"},
		{36, 1, "cursor x"},
		{40, 2, "cursor y"},
		{44, 3, "handle x"},
		{48, 4, "handle y"},
	} {
		if got := u32(tc.off); got != tc.want {
			t.Errorf("%s = %d, want %d", tc.name, got, tc.want)
		}
	}

	// Other formats still go through the img2txt-go codecs.
	if _, ok := cv.Export("utf8"); !ok {
		t.Error("utf8 export failed")
	}
	if _, ok := cv.Export("nonsense"); ok {
		t.Error("an unknown format was accepted")
	}
}
