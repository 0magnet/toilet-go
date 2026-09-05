// Command toilet renders text with FIGlet and TOIlet fonts.
//
// It is a Go port of TOIlet 0.3 by Sam Hocevar and takes the same command line.
package main

import (
	"fmt"
	"io"
	"math"
	"os"

	"github.com/0magnet/img2txt-go/caca"
	"github.com/0magnet/toilet-go/toilet"
)

// Date is the build date the original reports next to its version. TOIlet's own
// build leaves it blank, and this matches.
const Date = "  "

func main() {
	os.Exit(run(os.Args, os.Stdin, os.Stdout, os.Stderr))
}

// longOpts is toilet's long option table. The values above 127 are the private
// codes main.c uses for the options that have no short form.
var longOpts = []longOpt{
	{"font", true, 'f'},
	{"directory", true, 'd'},
	{"width", true, 'w'},
	{"termwidth", false, 't'},
	{"filter", true, 'F'},
	{"gay", false, 130},
	{"metal", false, 131},
	{"rainbow", false, 132},
	{"export", true, 'E'},
	{"irc", false, 140},
	{"html", false, 141},
	{"help", false, 'h'},
	{"infocode", true, 'I'},
	{"version", false, 'v'},
}

// run is main() with its streams passed in, so the tests can drive it.
func run(argv []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cx := toilet.New()
	infocode := -1

	g := newGetopt(argv, "f:d:w:tsSkWoF:E:hI:v", longOpts, stderr)

	for {
		c := g.next()
		if c == optEnd {
			break
		}

		switch c {
		case 'h': // --help
			fmt.Fprint(stdout, help, usage) //nolint:errcheck,gosec
			return 0
		case 'I': // --infocode
			infocode = atoi(g.arg)
		case 'v': // --version
			fmt.Fprintf(stdout, version, toilet.Version, Date, usage) //nolint:errcheck,gosec
			return 0
		case 'f': // --font
			cx.Font = g.arg
		case 'd': // --directory
			cx.Dir = g.arg
		case 'F': // --filter
			if g.arg == "list" {
				fmt.Fprintln(stdout, "Available filters:") //nolint:errcheck,gosec
				for _, f := range toilet.FilterList() {
					fmt.Fprintf(stdout, "%q: %s\n", f[0], f[1]) //nolint:errcheck,gosec
				}
				return 0
			}
			if err := cx.AddFilter(g.arg); err != nil {
				fmt.Fprintln(stderr, err) //nolint:errcheck,gosec
				return 255
			}
		// These four take names this program chose itself, so a failure would
		// be a bug here rather than bad input. The user-supplied filter above
		// is checked, because that one can genuinely be wrong.
		case 130, 132: // --gay, --rainbow
			_ = cx.AddFilter("rainbow") //nolint:errcheck
		case 131: // --metal
			_ = cx.AddFilter("metal") //nolint:errcheck
		case 'w': // --width
			cx.TermWidth = atoi(g.arg)
		case 't': // --termwidth
			if cols, ok := terminalWidth(); ok {
				cx.TermWidth = cols
			}
		case 's':
			cx.Mode = "default"
		case 'S':
			cx.Mode = "smush"
		case 'k':
			cx.Mode = "kern"
		case 'W':
			cx.Mode = "none"
		case 'o':
			cx.Mode = "overlap"
		case 'E': // --export
			if g.arg == "list" {
				fmt.Fprintln(stdout, "Available export formats:") //nolint:errcheck,gosec
				for _, e := range caca.ExportList() {
					fmt.Fprintf(stdout, "%q: %s\n", e[0], e[1]) //nolint:errcheck,gosec
				}
				return 0
			}
			if err := cx.SetExport(g.arg); err != nil {
				fmt.Fprintln(stderr, err) //nolint:errcheck,gosec
				return 255
			}
		case 140: // --irc
			_ = cx.SetExport("irc") //nolint:errcheck
		case 141: // --html
			_ = cx.SetExport("html") //nolint:errcheck
		case optBad:
			fmt.Fprintf(stdout, "Try `%s --help' for more information.\n", g.prog) //nolint:errcheck,gosec
			return 1
		}
	}

	switch infocode {
	case -1:
	case 0:
		fmt.Fprintf(stdout, version, toilet.Version, Date, usage) //nolint:errcheck,gosec
		return 0
	case 1:
		fmt.Fprintln(stdout, "20201") //nolint:errcheck,gosec
		return 0
	case 2:
		fmt.Fprintln(stdout, cx.Dir) //nolint:errcheck,gosec
		return 0
	case 3:
		fmt.Fprintln(stdout, cx.Font) //nolint:errcheck,gosec
		return 0
	case 4:
		// term_width is an unsigned int in the original, so a width set
		// negative comes back as a large positive number.
		fmt.Fprintln(stdout, uint32(cx.TermWidth)) //nolint:errcheck,gosec
		return 0
	default:
		return 0
	}

	if err := cx.Init(); err != nil {
		fmt.Fprintln(stderr, err) //nolint:errcheck,gosec
		return 255
	}

	if err := cx.Render(g.args(), stdin, stdout); err != nil {
		fmt.Fprintln(stderr, err) //nolint:errcheck,gosec
		return 255
	}

	return 0
}

const usage = "Usage: toilet [ -hkostvSW ] [ -d fontdirectory ]\n" +
	"              [ -f fontfile ] [ -F filter ] [ -w outputwidth ]\n" +
	"              [ -I infocode ] [ -E format ] [ message ]\n"

const help = "  -f, --font <name>        select the font\n" +
	"  -d, --directory <dir>    specify font directory\n" +
	"  -s, -S, -k, -W, -o       render mode (default, force smushing,\n" +
	"                           kerning, full width, overlap)\n" +
	"  -w, --width <width>      set output width\n" +
	"  -t, --termwidth          adapt to terminal's width\n" +
	"  -F, --filter <filters>   apply one or several filters to the text\n" +
	"  -F, --filter list        list available filters\n" +
	"      --rainbow            rainbow filter (same as -F rainbow)\n" +
	"      --metal              metal filter (same as -F metal)\n" +
	"  -E, --export <format>    select export format\n" +
	"  -E, --export list        list available export formats\n" +
	"      --irc                output IRC colour codes (same as -E irc)\n" +
	"      --html               output an HTML document (same as -E html)\n" +
	"  -h, --help               display this help and exit\n" +
	"  -I, --infocode <code>    print FIGlet-compatible infocode\n" +
	"  -v, --version            output version information and exit\n"

const version = "TOIlet Copyright 2006 Sam Hocevar\n" +
	"Internet: <sam@hocevar.net> Version: %s, date: %s\n" +
	"\n" +
	"TOIlet, along with the various TOIlet fonts and documentation, may be\n" +
	"freely copied and distributed.\n" +
	"\n" +
	"If you use TOIlet, please send an e-mail message to <sam@hocevar.net>.\n" +
	"\n" +
	"The latest version of TOIlet is available from the web site,\n" +
	"        http://libcaca.zoy.org/toilet.html\n" +
	"\n" +
	"%s"

// atoi is C's atoi(): it reads the leading integer and ignores whatever
// follows, giving zero when there is no integer at all. The result is
// truncated to 32 bits, as it is when glibc casts strtol's long down to an int,
// which is what makes "-w 99999999999999" a width of 276447231.
func atoi(s string) int {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' ||
		s[i] == '\v' || s[i] == '\f' || s[i] == '\r') {
		i++
	}

	neg := false
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		neg = s[i] == '-'
		i++
	}

	var v int64
	overflow := false
	for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		d := int64(s[i] - '0')
		if overflow {
			continue
		}
		if v > (math.MaxInt64-d)/10 {
			overflow = true
			continue
		}
		v = v*10 + d
	}

	if overflow {
		// strtol stops at LONG_MAX or LONG_MIN, whose low words are -1 and 0.
		if neg {
			return 0
		}
		return -1
	}

	if neg {
		v = -v
	}

	return int(int32(v))
}
