package toilet

import (
	"bytes"
	"strings"
	"testing"

	"github.com/0magnet/toilet-go/fonts"
)

func TestAddFilter(t *testing.T) {
	cases := []struct {
		spec    string
		want    []string
		wantErr bool
	}{
		{"crop", []string{"crop"}, false},
		{"crop:rainbow", []string{"crop", "rainbow"}, false},
		{":::crop:::", []string{"crop"}, false},
		{"", nil, false},
		{":::", nil, false},
		// The table is scanned backwards, so "180" is found before "left"
		// would be tried and "rotate" is not shadowed by "right".
		{"180", []string{"180"}, false},
		{"rotate", []string{"rotate"}, false},
		{"right:left:180", []string{"right", "left", "180"}, false},
		// A name that is only a prefix of a real filter is rejected.
		{"flipx", nil, true},
		{"cro", nil, true},
		{"bogus", nil, true},
		{"crop:bogus", nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.spec, func(t *testing.T) {
			c := New()
			err := c.AddFilter(tc.spec)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(c.Filters, ",") != strings.Join(tc.want, ",") {
				t.Errorf("filters = %q, want %q", c.Filters, tc.want)
			}
		})
	}
}

func TestFilterList(t *testing.T) {
	list := FilterList()

	// The hidden "rotate" alias is not advertised.
	for _, f := range list {
		if f[0] == "rotate" {
			t.Error("the rotate alias should not be listed")
		}
		if f[1] == "" {
			t.Errorf("filter %q has no description", f[0])
		}
	}
	if len(list) != 9 {
		t.Errorf("listed %d filters, want 9", len(list))
	}
	if list[0][0] != "crop" || list[len(list)-1][0] != "border" {
		t.Errorf("list starts with %q and ends with %q", list[0][0], list[len(list)-1][0])
	}
}

func TestSetExport(t *testing.T) {
	c := New()

	for _, f := range []string{"caca", "utf8", "irc", "tga", "troff"} {
		if err := c.SetExport(f); err != nil {
			t.Errorf("SetExport(%q): %v", f, err)
		}
	}
	if err := c.SetExport("bogus"); err == nil {
		t.Error("an unknown format was accepted")
	}
}

func TestEmbeddedFontsAreComplete(t *testing.T) {
	// The bundle is the set TOIlet installs.
	names := fonts.List()
	if len(names) != 24 {
		t.Errorf("bundled %d fonts, want 24", len(names))
	}

	for _, n := range names {
		if _, ok := fonts.Get(n); !ok {
			t.Errorf("fonts.Get(%q) failed", n)
		}
	}
	if _, ok := fonts.Get("nosuchfont"); ok {
		t.Error("a font that is not bundled was found")
	}
	// A name with a path separator must not escape the bundle.
	if _, ok := fonts.Get("../fonts/smblock"); ok {
		t.Error("a path was accepted as a font name")
	}
}

// renderTo runs a context over the given arguments and returns what it wrote.
func renderTo(t *testing.T, c *Context, args []string, stdin string) string {
	t.Helper()

	c.Dir = "../fonts"
	var out bytes.Buffer
	if err := c.Render(args, strings.NewReader(stdin), &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestRenderFromArguments(t *testing.T) {
	c := New()
	c.Font = "smblock"

	got := renderTo(t, c, []string{"Hi"}, "")
	if got == "" || !strings.HasSuffix(got, "\n") {
		t.Fatalf("render produced %q", got)
	}
	// smblock is four rows tall.
	if n := strings.Count(got, "\n"); n != 4 {
		t.Errorf("got %d rows, want 4", n)
	}
}

func TestRenderFromStdin(t *testing.T) {
	c := New()
	c.Font = "smblock"

	got := renderTo(t, c, nil, "ab\ncd\n")
	if n := strings.Count(got, "\n"); n != 8 {
		t.Errorf("got %d rows for two lines, want 8", n)
	}
}

func TestRenderTermFont(t *testing.T) {
	c := New()
	c.Font = "term"

	got := renderTo(t, c, []string{"hello"}, "")
	if got != "hello\n" {
		t.Errorf("term font rendered %q, want %q", got, "hello\n")
	}

	// The term font is the only one that carries the input's own colours.
	c2 := New()
	c2.Font = "term"
	coloured := renderTo(t, c2, nil, "\033[31mred\033[0m\n")
	if !strings.Contains(coloured, "\033[") {
		t.Errorf("term font dropped the input colours: %q", coloured)
	}
}

func TestRenderJoinsArgumentsWithSpaces(t *testing.T) {
	c := New()
	c.Font = "term"

	if got := renderTo(t, c, []string{"a", "b", "c"}, ""); got != "a b c\n" {
		t.Errorf("got %q, want %q", got, "a b c\n")
	}
}

func TestRenderSplitsOnNewlines(t *testing.T) {
	c := New()
	c.Font = "term"

	if got := renderTo(t, c, []string{"one\ntwo"}, ""); got != "one\ntwo\n" {
		t.Errorf("got %q, want %q", got, "one\ntwo\n")
	}
}

func TestRenderStdinNULTruncatesTheLine(t *testing.T) {
	// The original measures each line it reads with strlen().
	c := New()
	c.Font = "term"

	if got := renderTo(t, c, nil, "ab\x00cd\n"); got != "ab\n" {
		t.Errorf("got %q, want %q", got, "ab\n")
	}
}

func TestRenderWidthWraps(t *testing.T) {
	c := New()
	c.Font = "term"
	c.TermWidth = 4

	if got := renderTo(t, c, []string{"abcdefgh"}, ""); got != "abcd\nefgh\n" {
		t.Errorf("got %q, want %q", got, "abcd\nefgh\n")
	}
}

func TestRenderFilters(t *testing.T) {
	c := New()
	c.Font = "term"
	if err := c.AddFilter("border"); err != nil {
		t.Fatal(err)
	}

	got := renderTo(t, c, []string{"ab"}, "")
	want := "┌──┐\n│ab│\n└──┘\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderExportFormat(t *testing.T) {
	c := New()
	c.Font = "term"
	if err := c.SetExport("html"); err != nil {
		t.Fatal(err)
	}

	got := renderTo(t, c, []string{"x"}, "")
	if !strings.HasPrefix(got, "<!DOCTYPE html") {
		t.Errorf("html export started with %q", got[:min(20, len(got))])
	}
}

func TestInitFallsBackToTheBundledFonts(t *testing.T) {
	// With a font directory that does not exist, the embedded collection is
	// used. This is the one place where the port does more than the original.
	c := New()
	c.Font = "smblock"
	c.Dir = "/nonexistent"

	if err := c.Init(); err != nil {
		t.Fatalf("bundled font not found: %v", err)
	}

	c2 := New()
	c2.Font = "nosuchfont"
	c2.Dir = "/nonexistent"
	if err := c2.Init(); err == nil {
		t.Error("a font that exists nowhere was loaded")
	}
}
