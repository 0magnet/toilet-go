package main

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// update regenerates the golden files from the system toilet. Run it as
//
//	go test ./cmd/toilet -update
//
// on a machine with toilet 0.3 and libcaca 0.99.beta20 installed. The files it
// writes are the C program's own output, so the tests below check this port
// against the original rather than against itself.
var update = flag.Bool("update", false, "regenerate testdata from /usr/bin/toilet")

// fontDir is the bundled font collection, which is a byte-for-byte copy of the
// one TOIlet installs, so the reference binary can be pointed at it too.
const fontDir = "../../fonts"

// goldenCases are the command lines the golden files cover.
var goldenCases = []struct {
	name  string
	args  []string
	stdin string
}{
	{"default-font", []string{"Hi"}, ""},
	{"smblock", []string{"-f", "smblock", "Hi there"}, ""},
	{"future", []string{"-f", "future", "Hello, World!"}, ""},
	{"pagga", []string{"-f", "pagga", "pagga"}, ""},
	{"emboss", []string{"-f", "emboss", "emboss"}, ""},
	{"letter", []string{"-f", "letter", "ABC"}, ""},
	{"wideterm", []string{"-f", "wideterm", "wide"}, ""},
	{"smbraille", []string{"-f", "smbraille", "braille"}, ""},
	{"bfraktur", []string{"-f", "bfraktur", "fraktur"}, ""},
	{"circle", []string{"-f", "circle", "circle"}, ""},
	{"fauxcyrillic", []string{"-f", "fauxcyrillic", "faux"}, ""},
	{"mono9", []string{"-f", "mono9", "mono"}, ""},
	{"bigmono9", []string{"-f", "bigmono9", "big"}, ""},
	{"term", []string{"-f", "term", "term font"}, ""},

	{"mode-default", []string{"-f", "smblock", "-s", "AVWAVW"}, ""},
	{"mode-smush", []string{"-f", "smblock", "-S", "AVWAVW"}, ""},
	{"mode-kern", []string{"-f", "smblock", "-k", "AVWAVW"}, ""},
	{"mode-fullwidth", []string{"-f", "smblock", "-W", "AVWAVW"}, ""},
	{"mode-overlap", []string{"-f", "smblock", "-o", "AVWAVW"}, ""},

	{"width-10", []string{"-f", "smblock", "-w", "10", "wrapping is fun"}, ""},
	{"width-1", []string{"-f", "smblock", "-w", "1", "narrow"}, ""},
	{"width-200", []string{"-f", "smblock", "-w", "200", "wide load here"}, ""},

	{"filter-crop", []string{"-f", "smblock", "-F", "crop", "crop"}, ""},
	{"filter-rainbow", []string{"-f", "smblock", "-F", "rainbow", "rainbow"}, ""},
	{"filter-metal", []string{"-f", "smblock", "-F", "metal", "metal"}, ""},
	{"filter-flip", []string{"-f", "smblock", "-F", "flip", "flip"}, ""},
	{"filter-flop", []string{"-f", "smblock", "-F", "flop", "flop"}, ""},
	{"filter-180", []string{"-f", "smblock", "-F", "180", "180"}, ""},
	{"filter-left", []string{"-f", "smblock", "-F", "left", "left"}, ""},
	{"filter-right", []string{"-f", "smblock", "-F", "right", "right"}, ""},
	{"filter-border", []string{"-f", "smblock", "-F", "border", "border"}, ""},
	{"filter-rotate-alias", []string{"-f", "smblock", "-F", "rotate", "rot"}, ""},
	{"filter-chain", []string{"-f", "smblock", "-F", "crop:rainbow:border", "chain"}, ""},
	{"filter-gay", []string{"-f", "smblock", "--gay", "gay"}, ""},
	{"filter-metal-long", []string{"-f", "smblock", "--metal", "metal"}, ""},

	{"export-caca", []string{"-f", "smblock", "-E", "caca", "Hi"}, ""},
	{"export-ansi", []string{"-f", "smblock", "-E", "ansi", "Hi"}, ""},
	{"export-utf8cr", []string{"-f", "smblock", "-E", "utf8cr", "Hi"}, ""},
	{"export-html", []string{"-f", "smblock", "-E", "html", "Hi"}, ""},
	{"export-html3", []string{"-f", "smblock", "-E", "html3", "Hi"}, ""},
	{"export-bbfr", []string{"-f", "smblock", "-E", "bbfr", "Hi"}, ""},
	{"export-irc", []string{"-f", "smblock", "-E", "irc", "Hi"}, ""},
	{"export-ps", []string{"-f", "smblock", "-E", "ps", "Hi"}, ""},
	{"export-svg", []string{"-f", "smblock", "-E", "svg", "Hi"}, ""},
	{"export-irc-long", []string{"-f", "smblock", "--irc", "Hi"}, ""},
	{"export-html-long", []string{"-f", "smblock", "--html", "Hi"}, ""},

	{"args-several", []string{"-f", "smblock", "a", "b", "c"}, ""},
	{"args-newline", []string{"-f", "smblock", "one\ntwo"}, ""},
	{"args-empty", []string{"-f", "smblock", ""}, ""},

	{"stdin-lines", []string{"-f", "smblock"}, "one\ntwo\nthree\n"},
	{"stdin-no-newline", []string{"-f", "smblock"}, "trailing"},
	{"stdin-colors", []string{"-f", "term"}, "\033[31mred\033[0m plain\n"},
	{"stdin-utf8", []string{"-f", "term"}, "café äöü ☃\n"},
	{"stdin-fullwidth", []string{"-f", "term"}, "日本語\n"},
	{"stdin-tab", []string{"-f", "term"}, "a\tb\n"},
	{"stdin-empty-line", []string{"-f", "smblock"}, "\n"},
	{"stdin-rainbow-two-lines", []string{"-f", "smblock", "-F", "rainbow"}, "ab\ncd\n"},

	{"help", []string{"-h"}, ""},
	{"version", []string{"-v"}, ""},
	{"export-list", []string{"-E", "list"}, ""},
	{"filter-list", []string{"-F", "list"}, ""},
	{"infocode-1", []string{"-I", "1"}, ""},
	{"infocode-3", []string{"-I", "3", "-f", "future"}, ""},
	{"infocode-4", []string{"-I", "4", "-w", "42"}, ""},
	{"infocode-junk", []string{"-I", "3x", "-f", "future"}, ""},
	{"bad-option", []string{"-z", "Hi"}, ""},
	{"bad-long-option", []string{"--nope", "Hi"}, ""},
	{"missing-argument", []string{"-f"}, ""},
	{"bad-filter", []string{"-F", "bogus", "Hi"}, ""},
	{"bad-export", []string{"-E", "bogus", "Hi"}, ""},
	{"missing-font", []string{"-f", "nosuchfont", "Hi"}, ""},
	{"permuted-options", []string{"Hi", "-f", "future"}, ""},
	{"clustered-options", []string{"-f", "smblock", "-tW", "Hi"}, ""},
	{"attached-argument", []string{"-fsmblock", "Hi"}, ""},
	{"long-with-equals", []string{"--font=future", "Hi"}, ""},
	{"end-of-options", []string{"-f", "smblock", "--", "-x"}, ""},
}

// goldenArgs prepends the program name and the font directory.
func goldenArgs(args []string) []string {
	return append([]string{"toilet", "-d", fontDir}, args...)
}

func TestGolden(t *testing.T) {
	if *update {
		regenerate(t)
	}

	for _, tc := range goldenCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join("testdata", tc.name)
			want, err := os.ReadFile(path) //nolint:gosec
			if err != nil {
				t.Fatalf("%v (run `go test ./cmd/toilet -update` with toilet installed)", err)
			}

			var stdout, stderr bytes.Buffer
			code := run(goldenArgs(tc.args), strings.NewReader(tc.stdin), &stdout, &stderr)

			if got := encode(stdout.Bytes(), stderr.Bytes(), code); !bytes.Equal(got, want) {
				t.Errorf("output differs from %s\n got %q\nwant %q", path, got, want)
			}
		})
	}
}

// encode packs a run's three results into one golden file: the exit status on
// the first line, then stdout and stderr with a marker between them.
func encode(stdout, stderr []byte, code int) []byte {
	var b bytes.Buffer
	b.WriteString("exit " + strconv.Itoa(code) + "\n--- stdout\n")
	b.Write(stdout)
	b.WriteString("--- stderr\n")
	b.Write(stderr)
	return b.Bytes()
}

// regenerate rewrites every golden file from the system toilet.
func regenerate(t *testing.T) {
	t.Helper()

	if err := os.MkdirAll("testdata", 0o750); err != nil {
		t.Fatal(err)
	}

	for _, tc := range goldenCases {
		cmd := exec.Command("/usr/bin/toilet", goldenArgs(tc.args)[1:]...) //nolint:gosec
		cmd.Args[0] = "toilet"                                             // the program names itself in its error messages
		cmd.Stdin = strings.NewReader(tc.stdin)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		_ = cmd.Run() //nolint:errcheck

		code := cmd.ProcessState.ExitCode()
		if code < 0 {
			t.Fatalf("%s: the reference binary died on a signal", tc.name)
		}

		path := filepath.Join("testdata", tc.name)
		if err := os.WriteFile(path, encode(stdout.Bytes(), stderr.Bytes(), code), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
}

func TestAtoi(t *testing.T) {
	// C's atoi, which reads a leading integer, ignores the rest and truncates
	// to 32 bits.
	cases := []struct {
		in   string
		want int
	}{
		{"0", 0},
		{"42", 42},
		{" 42", 42},
		{"-5", -5},
		{"+5", 5},
		{"3x", 3},
		{"9;", 9},
		{"abc", 0},
		{"", 0},
		{"4294967291", -5},
		{"99999999999999", 276447231},
		{"9999999999999999999999999", -1},
		{"-9999999999999999999999999", 0},
	}

	for _, tc := range cases {
		if got := atoi(tc.in); got != tc.want {
			t.Errorf("atoi(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestGetopt(t *testing.T) {
	longs := []longOpt{
		{"font", true, 'f'},
		{"help", false, 'h'},
		{"filter", true, 'F'},
	}

	t.Run("permutes non-options", func(t *testing.T) {
		g := newGetopt([]string{"toilet", "Hi", "-f", "mini", "there"},
			"f:h", longs, &bytes.Buffer{})

		if c := g.next(); c != 'f' || g.arg != "mini" {
			t.Fatalf("got option %q with %q, want 'f' with \"mini\"", c, g.arg)
		}
		if c := g.next(); c != optEnd {
			t.Fatalf("got option %q, want the end", c)
		}
		if args := g.args(); len(args) != 2 || args[0] != "Hi" || args[1] != "there" {
			t.Errorf("operands = %q, want [Hi there]", args)
		}
	})

	t.Run("clusters and attached values", func(t *testing.T) {
		g := newGetopt([]string{"toilet", "-hfmini"}, "f:h", longs, &bytes.Buffer{})

		if c := g.next(); c != 'h' {
			t.Fatalf("got %q, want 'h'", c)
		}
		if c := g.next(); c != 'f' || g.arg != "mini" {
			t.Fatalf("got %q with %q, want 'f' with \"mini\"", c, g.arg)
		}
	})

	t.Run("long options", func(t *testing.T) {
		g := newGetopt([]string{"toilet", "--font=mini", "--fo", "big", "--help"},
			"f:h", longs, &bytes.Buffer{})

		if c := g.next(); c != 'f' || g.arg != "mini" {
			t.Fatalf("--font=mini gave %q with %q", c, g.arg)
		}
		if c := g.next(); c != 'f' || g.arg != "big" {
			t.Fatalf("--fo big gave %q with %q", c, g.arg)
		}
		if c := g.next(); c != 'h' {
			t.Fatalf("--help gave %q", c)
		}
	})

	t.Run("double dash ends the options", func(t *testing.T) {
		g := newGetopt([]string{"toilet", "--", "-f", "x"}, "f:h", longs, &bytes.Buffer{})

		if c := g.next(); c != optEnd {
			t.Fatalf("got %q, want the end", c)
		}
		if args := g.args(); len(args) != 2 || args[0] != "-f" {
			t.Errorf("operands = %q, want [-f x]", args)
		}
	})

	t.Run("errors", func(t *testing.T) {
		var errs bytes.Buffer
		g := newGetopt([]string{"toilet", "-z"}, "f:h", longs, &errs)
		if c := g.next(); c != optBad {
			t.Fatalf("unknown option gave %q", c)
		}
		if !strings.Contains(errs.String(), "invalid option -- 'z'") {
			t.Errorf("message = %q", errs.String())
		}

		errs.Reset()
		g = newGetopt([]string{"toilet", "-f"}, "f:h", longs, &errs)
		if c := g.next(); c != optBad {
			t.Fatalf("missing argument gave %q", c)
		}
		if !strings.Contains(errs.String(), "requires an argument") {
			t.Errorf("message = %q", errs.String())
		}

		// A non-ASCII byte is reported as the raw byte, as getopt reports it.
		errs.Reset()
		g = newGetopt([]string{"toilet", "-ü"}, "f:h", longs, &errs)
		if c := g.next(); c != optBad {
			t.Fatalf("non-ASCII option gave %q", c)
		}
		if got := errs.String(); !strings.Contains(got, "-- '\xc3'") {
			t.Errorf("message = %q, want the raw byte", got)
		}
	})
}
