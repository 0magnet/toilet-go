package canvas

import "testing"

// The expected strings here were taken from the C program: a two-row synthetic
// font was rendered by toilet 0.3 with each filter in turn.

func TestFlip(t *testing.T) {
	cv := fill("ab(", "/_\\")
	cv.Flip()

	// Mirrored characters are substituted where one exists: ( becomes ), b
	// becomes d, and the slashes swap, which leaves "/_\" looking the same.
	want := []string{")da", "/_\\"}
	for i, got := range rows(cv) {
		if got != want[i] {
			t.Errorf("row %d = %q, want %q", i, got, want[i])
		}
	}
}

func TestFlop(t *testing.T) {
	cv := fill("bq", "MW")
	cv.Flop()

	// Flopping swaps the rows and each character with its vertical mirror.
	want := []string{"WM", "pd"}
	for i, got := range rows(cv) {
		if got != want[i] {
			t.Errorf("row %d = %q, want %q", i, got, want[i])
		}
	}
}

func TestRotate180(t *testing.T) {
	cv := fill("ab", "cd")
	cv.Rotate180()

	want := []string{"pɔ", "qɐ"}
	for i, got := range rows(cv) {
		if got != want[i] {
			t.Errorf("row %d = %q, want %q", i, got, want[i])
		}
	}
}

func TestFlipFlopRotateAreInvolutive(t *testing.T) {
	// The three of them are documented as involutive: applying one twice must
	// give back the original.
	src := []string{"|/\\[]{}()<>_-", "abcdefghijklm", "ABCDEFGHIJKLM"}

	for _, tc := range []struct {
		name string
		fn   func(*Canvas)
	}{
		{"Flip", (*Canvas).Flip},
		{"Flop", (*Canvas).Flop},
		{"Rotate180", (*Canvas).Rotate180},
	} {
		cv := fill(src...)
		tc.fn(cv)
		tc.fn(cv)
		for i, got := range rows(cv) {
			if got != src[i] {
				t.Errorf("%s twice: row %d = %q, want %q", tc.name, i, got, src[i])
			}
		}
	}
}

func TestRotateLeftAndRight(t *testing.T) {
	// Cells turn two by two, so a 4x2 canvas becomes 4x2 again: the width
	// halves and becomes the height, the height doubles and becomes the width.
	cv := fill("ab-|", "cd|-")

	cv.RotateLeft()
	if cv.Width != 4 || cv.Height != 2 {
		t.Fatalf("after RotateLeft: %dx%d, want 4x2", cv.Width, cv.Height)
	}
	// The right-hand pair "-|" and "|-" rotate into each other's characters.
	if got := rows(cv); got[0] != "-||-" || got[1] != "abcd" {
		t.Errorf("RotateLeft = %q, want [-||- abcd]", got)
	}

	cv2 := fill("ab-|", "cd|-")
	cv2.RotateRight()
	if cv2.Width != 4 || cv2.Height != 2 {
		t.Fatalf("after RotateRight: %dx%d, want 4x2", cv2.Width, cv2.Height)
	}
	if got := rows(cv2); got[0] != "cdab" || got[1] != "|--|" {
		t.Errorf("RotateRight = %q, want [cdab |--|]", got)
	}
}

func TestRotateOddWidthLosesTheLastColumn(t *testing.T) {
	// An odd width is rounded up and the missing cell is treated as a space,
	// so the last column is dropped.
	cv := fill("abc")
	cv.RotateLeft()

	if cv.Width != 2 || cv.Height != 2 {
		t.Fatalf("size = %dx%d, want 2x2", cv.Width, cv.Height)
	}
}

func TestRotateEmptyCanvasIsANoOp(t *testing.T) {
	// libcaca's allocator refuses a zero dimension, so its rotate functions
	// bail out before touching anything — cursor and handle included.
	for _, fn := range []func(*Canvas){(*Canvas).RotateLeft, (*Canvas).RotateRight} {
		cv := New(0, 0)
		cv.X, cv.Y = 3, 4
		fn(cv)
		if cv.Width != 0 || cv.Height != 0 || cv.X != 3 || cv.Y != 4 {
			t.Errorf("rotating an empty canvas changed it: %dx%d at %d,%d",
				cv.Width, cv.Height, cv.X, cv.Y)
		}
	}
}

func TestRotateMovesTheCursor(t *testing.T) {
	cv := fill("abcd", "efgh")
	cv.X, cv.Y = 1, 1
	cv.HandleX, cv.HandleY = 2, 0

	cv.RotateLeft()
	// x becomes y*2 and y becomes (oldWidth-1-x)/2.
	if cv.X != 2 || cv.Y != 1 {
		t.Errorf("cursor after RotateLeft = %d,%d; want 2,1", cv.X, cv.Y)
	}
	if cv.HandleX != 0 || cv.HandleY != 0 {
		t.Errorf("handle after RotateLeft = %d,%d; want 0,0", cv.HandleX, cv.HandleY)
	}

	cv2 := fill("abcd", "efgh")
	cv2.X, cv2.Y = 1, 1
	cv2.RotateRight()
	// x becomes (oldHeight-1-y)*2 and y becomes x/2.
	if cv2.X != 0 || cv2.Y != 0 {
		t.Errorf("cursor after RotateRight = %d,%d; want 0,0", cv2.X, cv2.Y)
	}
}
