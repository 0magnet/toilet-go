package main

import (
	"fmt"
	"io"
	"strings"
)

// This is enough of GNU getopt_long() to parse toilet's command line the way
// the C program does, permutation of non-option arguments included.

// longOpt describes one long option.
type longOpt struct {
	name   string
	hasArg bool
	val    int
}

// getopt walks a command line one option at a time.
type getopt struct {
	argv    []string
	optstr  string
	longs   []longOpt
	stderr  io.Writer
	prog    string
	ind     int      // index of the next argument to look at
	sub     int      // offset inside a cluster of short options
	operand []string // non-option arguments, collected as they are passed
	arg     string   // argument of the option just returned
	done    bool
}

// results returned by next besides an option value.
const (
	optEnd = -1
	optBad = '?'
)

// newGetopt returns a parser for argv, whose first element is the program name.
func newGetopt(argv []string, optstr string, longs []longOpt, stderr io.Writer) *getopt {
	prog := "toilet"
	if len(argv) > 0 {
		prog = argv[0]
	}
	return &getopt{argv: argv, optstr: optstr, longs: longs, stderr: stderr,
		prog: prog, ind: 1}
}

// next returns the next option, optEnd when the options run out, or optBad for
// one that could not be parsed.
func (g *getopt) next() int {
	g.arg = ""

	if g.sub == 0 {
		// Skip over and remember everything that is not an option.
		for g.ind < len(g.argv) {
			a := g.argv[g.ind]
			if a == "--" {
				g.ind++
				g.done = true
				break
			}
			if len(a) >= 2 && a[0] == '-' {
				break
			}
			g.operand = append(g.operand, a)
			g.ind++
		}
		if g.done {
			g.operand = append(g.operand, g.argv[g.ind:]...)
			g.ind = len(g.argv)
		}
		if g.ind >= len(g.argv) {
			return optEnd
		}
	}

	a := g.argv[g.ind]

	if g.sub == 0 && strings.HasPrefix(a, "--") {
		return g.longOption(a)
	}
	return g.shortOption(a)
}

// longOption handles one "--name" or "--name=value" argument.
func (g *getopt) longOption(a string) int {
	body := a[2:]
	name, value := body, ""
	hasValue := false
	if i := strings.IndexByte(body, '='); i >= 0 {
		name, value, hasValue = body[:i], body[i+1:], true
	}

	// An exact match wins; failing that, a unique prefix does.
	match := -1
	for i, l := range g.longs {
		if l.name == name {
			match = i
			break
		}
		if strings.HasPrefix(l.name, name) {
			if match >= 0 {
				fmt.Fprintf(g.stderr, "%s: option '%s' is ambiguous\n", g.prog, a)
				g.ind++
				return optBad
			}
			match = i
		}
	}

	if match < 0 {
		fmt.Fprintf(g.stderr, "%s: unrecognized option '%s'\n", g.prog, a)
		g.ind++
		return optBad
	}

	l := g.longs[match]
	g.ind++

	switch {
	case l.hasArg && hasValue:
		g.arg = value
	case l.hasArg:
		if g.ind >= len(g.argv) {
			fmt.Fprintf(g.stderr, "%s: option '--%s' requires an argument\n",
				g.prog, l.name)
			return optBad
		}
		g.arg = g.argv[g.ind]
		g.ind++
	case hasValue:
		fmt.Fprintf(g.stderr, "%s: option '--%s' doesn't allow an argument\n",
			g.prog, l.name)
		return optBad
	}

	return l.val
}

// shortOption handles one character of a cluster such as "-tW".
func (g *getopt) shortOption(a string) int {
	if g.sub == 0 {
		g.sub = 1
	}

	c := a[g.sub]
	g.sub++
	if g.sub >= len(a) {
		g.sub = 0
		g.ind++
	}

	i := strings.IndexByte(g.optstr, c)
	if c == ':' || i < 0 {
		// The byte is written as it stands, not as a rune: an argument such
		// as "-ü" makes getopt complain about each of its bytes in turn.
		fmt.Fprintf(g.stderr, "%s: invalid option -- '%s'\n", g.prog, []byte{c})
		return optBad
	}

	if i+1 < len(g.optstr) && g.optstr[i+1] == ':' {
		switch {
		case g.sub != 0:
			// The rest of this argument is the option's value.
			g.arg = a[g.sub:]
			g.sub = 0
			g.ind++
		case g.ind < len(g.argv):
			g.arg = g.argv[g.ind]
			g.ind++
		default:
			fmt.Fprintf(g.stderr, "%s: option requires an argument -- '%s'\n",
				g.prog, []byte{c})
			return optBad
		}
	}

	return int(c)
}

// args returns the non-option arguments, in their original order.
func (g *getopt) args() []string {
	out := append([]string{}, g.operand...)
	return append(out, g.argv[g.ind:]...)
}
