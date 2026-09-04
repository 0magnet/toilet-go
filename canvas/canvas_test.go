package canvas

import (
	"strings"
	"testing"

	"github.com/0magnet/img2txt-go/caca"
)

// rows renders the canvas as one string per line.
func rows(cv *Canvas) []string {
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

// fill writes the given rows onto a canvas of matching size.
func fill(lines ...string) *Canvas {
	w := 0
	for _, l := range lines {
		if n := len([]rune(l)); n > w {
			w = n
		}
	}
	cv := New(w, len(lines))
	for y, l := range lines {
		for x, ch := range []rune(l) {
			cv.PutChar(x, y, ch)
		}
	}
	return cv
}

func TestNewCanvasDefaults(t *testing.T) {
	cv := New(3, 2)

	if cv.Width != 3 || cv.Height != 2 {
		t.Fatalf("size = %dx%d, want 3x2", cv.Width, cv.Height)
	}
	// caca_create_canvas() sets the default foreground on a transparent
	// background before it fills the cells.
	const wantAttr = 0x01800500
	if cv.Attr() != wantAttr {
		t.Errorf("attribute = %#x, want %#x", cv.Attr(), wantAttr)
	}
	for i, ch := range cv.Chars {
		if ch != ' ' {
			t.Fatalf("cell %d = %q, want a space", i, ch)
		}
		if cv.Attrs[i] != wantAttr {
			t.Fatalf("cell %d attribute = %#x, want %#x", i, cv.Attrs[i], wantAttr)
		}
	}
}

func TestSetSizePreservesContent(t *testing.T) {
	cases := []struct {
		name string
		w, h int
		want []string
	}{
		{"wider", 5, 2, []string{"abc  ", "def  "}},
		{"narrower", 2, 2, []string{"ab", "de"}},
		{"taller", 3, 4, []string{"abc", "def", "   ", "   "}},
		{"shorter", 3, 1, []string{"abc"}},
		{"wider and taller", 4, 3, []string{"abc ", "def ", "    "}},
		{"narrower and shorter", 2, 1, []string{"ab"}},
		{"to nothing", 0, 0, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cv := fill("abc", "def")
			cv.SetSize(tc.w, tc.h)

			got := rows(cv)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d rows %q, want %d %q",
					len(got), got, len(tc.want), tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("row %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestSetSizeGrowsFromNothing(t *testing.T) {
	cv := New(0, 0)
	cv.SetSize(2, 2)

	if got := rows(cv); got[0] != "  " || got[1] != "  " {
		t.Errorf("grown canvas = %q, want two blank rows", got)
	}
}

func TestPutCharBounds(t *testing.T) {
	cv := New(2, 2)

	// Out of bounds writes are dropped, not errors.
	cv.PutChar(-1, 0, 'x')
	cv.PutChar(0, -1, 'x')
	cv.PutChar(2, 0, 'x')
	cv.PutChar(0, 2, 'x')

	for _, ch := range cv.Chars {
		if ch != ' ' {
			t.Fatalf("an out-of-bounds write landed: %q", rows(cv))
		}
	}

	// Reads outside the canvas give a space.
	if cv.GetChar(-1, -1) != ' ' || cv.GetChar(9, 9) != ' ' {
		t.Error("a read outside the canvas did not give a space")
	}
	// Reads outside the canvas give the current attribute.
	if cv.GetAttr(9, 9) != cv.Attr() {
		t.Error("a read outside the canvas did not give the current attribute")
	}
}

func TestPutCharFullwidth(t *testing.T) {
	cv := New(4, 1)

	if n := cv.PutChar(0, 0, '日'); n != 2 {
		t.Errorf("PutChar of a fullwidth glyph returned %d, want 2", n)
	}
	if cv.GetChar(0, 0) != '日' || uint32(cv.GetChar(1, 0)) != caca.MagicFullwidth {
		t.Fatalf("fullwidth glyph did not claim two cells: %v", cv.Chars)
	}

	// Overwriting the left half blanks the right half.
	cv.PutChar(0, 0, 'x')
	if cv.GetChar(1, 0) != ' ' {
		t.Errorf("right half = %q, want a space", cv.GetChar(1, 0))
	}

	// Overwriting the right half blanks the left half.
	cv.PutChar(2, 0, '本')
	cv.PutChar(3, 0, 'y')
	if cv.GetChar(2, 0) != ' ' {
		t.Errorf("left half = %q, want a space", cv.GetChar(2, 0))
	}

	// A fullwidth glyph in the last column becomes a space: it does not fit.
	cv2 := New(1, 1)
	cv2.PutChar(0, 0, '日')
	if cv2.GetChar(0, 0) != ' ' {
		t.Errorf("fullwidth glyph in a one-column canvas = %q, want a space",
			cv2.GetChar(0, 0))
	}
}

func TestPutAttrFollowsFullwidth(t *testing.T) {
	cv := New(2, 1)
	cv.PutChar(0, 0, '日')

	cv.PutAttr(0, 0, 0x12345670)
	if cv.GetAttr(1, 0) != cv.GetAttr(0, 0) {
		t.Error("the two halves of a fullwidth glyph have different attributes")
	}

	// A value below 0x10 replaces only the style bits.
	before := cv.GetAttr(0, 0)
	cv.PutAttr(0, 0, 0x3)
	if got, want := cv.GetAttr(0, 0), (before&0xfffffff0)|0x3; got != want {
		t.Errorf("style-only attribute = %#x, want %#x", got, want)
	}
}

func TestBlit(t *testing.T) {
	dst := New(5, 3)
	src := fill("ab", "cd")

	dst.Blit(1, 1, src)
	if got := rows(dst); got[1] != " ab  " || got[2] != " cd  " {
		t.Errorf("blit = %q", got)
	}

	// The handle shifts the source.
	dst2 := New(5, 3)
	src.SetHandle(1, 1)
	dst2.Blit(1, 1, src)
	if got := rows(dst2); got[0] != "ab   " || got[1] != "cd   " {
		t.Errorf("blit with a handle = %q", got)
	}

	// A blit that falls entirely outside changes nothing.
	dst3 := New(2, 2)
	src.SetHandle(0, 0)
	dst3.Blit(9, 9, src)
	if got := rows(dst3); got[0] != "  " || got[1] != "  " {
		t.Errorf("out-of-range blit changed the canvas: %q", got)
	}
}

func TestSetBoundaries(t *testing.T) {
	cv := fill("abcd", "efgh", "ijkl")

	crop := fill("abcd", "efgh", "ijkl")
	crop.SetBoundaries(1, 1, 2, 2)
	if got := rows(crop); len(got) != 2 || got[0] != "fg" || got[1] != "jk" {
		t.Errorf("crop = %q, want [fg jk]", got)
	}

	grow := fill("ab")
	grow.SetBoundaries(-1, -1, 4, 3)
	if got := rows(grow); len(got) != 3 || got[1] != " ab " {
		t.Errorf("grow = %q", got)
	}

	// The attribute goes back to a fresh canvas' default, which is what makes
	// the border filter draw an uncoloured box after a color filter.
	cv.SetColorANSI(caca.LightRed, caca.Black)
	cv.SetBoundaries(0, 0, 2, 2)
	if cv.Attr() != New(1, 1).Attr() {
		t.Errorf("attribute after SetBoundaries = %#x, want the default %#x",
			cv.Attr(), New(1, 1).Attr())
	}
}

func TestDrawCP437Box(t *testing.T) {
	cv := New(4, 3)
	cv.DrawCP437Box(0, 0, 4, 3)

	want := []string{"┌──┐", "│  │", "└──┘"}
	for i, got := range rows(cv) {
		if got != want[i] {
			t.Errorf("row %d = %q, want %q", i, got, want[i])
		}
	}
}

func TestFillBoxAndDrawLine(t *testing.T) {
	cv := New(4, 3)
	cv.FillBox(1, 1, 2, 2, '#')
	if got := rows(cv); got[0] != "    " || got[1] != " ## " || got[2] != " ## " {
		t.Errorf("FillBox = %q", got)
	}

	cv2 := New(4, 3)
	cv2.DrawLine(0, 1, 3, 1, '-')
	cv2.DrawLine(2, 0, 2, 2, '|')
	if got := rows(cv2); got[0] != "  | " || got[1] != "--|-" || got[2] != "  | " {
		t.Errorf("DrawLine = %q", got)
	}
}
