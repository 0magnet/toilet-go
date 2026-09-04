package figlet

import (
	"fmt"
	"testing"
)

// allRules enables every smushing rule at once.
const allRules = RuleEqual | RuleUnderscore | RuleHierarchy |
	RuleOppositePair | RuleBigX | RuleHardblank

func TestSmushRule1Equal(t *testing.T) {
	cases := []struct {
		ch1, ch2 rune
		rule     int
		want     rune
	}{
		{'|', '|', RuleEqual, '|'},
		{'#', '#', RuleEqual, '#'},
		{'A', 'A', RuleEqual, 'A'},
		{'A', 'B', RuleEqual, 0},
		// The rule has to be enabled.
		{'|', '|', 0, 0},
		// A pair of hardblanks is the documented exception: they do not smush,
		// which is what keeps a hardblank column open.
		{0xa0, 0xa0, RuleEqual, 0},
		// Rule 1 is the only one that looks past U+007F.
		{'€', '€', RuleEqual, '€'},
		{'€', '£', allRules, 0},
	}

	for _, tc := range cases {
		if got := Smush(tc.ch1, tc.ch2, tc.rule); got != tc.want {
			t.Errorf("Smush(%q, %q, %#x) = %q, want %q",
				tc.ch1, tc.ch2, tc.rule, got, tc.want)
		}
	}
}

func TestSmushRule2Underscore(t *testing.T) {
	// An underscore yields to any of the border characters, either way round.
	for _, border := range hierarchy {
		if got := Smush('_', border, RuleUnderscore); got != border {
			t.Errorf("Smush('_', %q) = %q, want %q", border, got, border)
		}
		if got := Smush(border, '_', RuleUnderscore); got != border {
			t.Errorf("Smush(%q, '_') = %q, want %q", border, got, border)
		}
	}

	// Anything else leaves the underscore alone.
	for _, other := range "aA0 #~" {
		if got := Smush('_', other, RuleUnderscore); got != 0 {
			t.Errorf("Smush('_', %q) = %q, want no smush", other, got)
		}
	}
	if got := Smush('_', '|', 0); got != 0 {
		t.Error("rule 2 fired while disabled")
	}
}

func TestSmushRule3Hierarchy(t *testing.T) {
	// The classes, in increasing order of precedence.
	classes := [][]rune{
		{'|'},
		{'/', '\\'},
		{'[', ']'},
		{'{', '}'},
		{'(', ')'},
		{'<', '>'},
	}

	for i, lo := range classes {
		for j, hi := range classes {
			for _, a := range lo {
				for _, b := range hi {
					got := Smush(a, b, RuleHierarchy)
					var want rune
					switch {
					case i < j:
						want = b
					case i > j:
						want = a
					default:
						// Same class: rule 3 declines, and with only rule 3
						// enabled nothing else can fire.
						want = 0
					}
					if got != want {
						t.Errorf("Smush(%q, %q) = %q, want %q", a, b, got, want)
					}
				}
			}
		}
	}

	if got := Smush('[', '(', 0); got != 0 {
		t.Error("rule 3 fired while disabled")
	}
}

func TestSmushRule4OppositePairs(t *testing.T) {
	cases := []struct {
		ch1, ch2 rune
		want     rune
	}{
		{'[', ']', '|'},
		{']', '[', '|'},
		{'{', '}', '|'},
		{'}', '{', '|'},
		{'(', ')', '|'},
		{')', '(', '|'},
		// Mismatched pairs do not.
		{'[', '}', 0},
		{'(', ']', 0},
		{'<', '>', 0},
	}

	for _, tc := range cases {
		if got := Smush(tc.ch1, tc.ch2, RuleOppositePair); got != tc.want {
			t.Errorf("Smush(%q, %q, rule 4) = %q, want %q",
				tc.ch1, tc.ch2, got, tc.want)
		}
	}

	// The parenthesis case is tested by product and sum together, because the
	// product alone is ambiguous. 0x28 * 0x29 is 1640, and so is 8 * 205; the
	// sum check is what rejects the impostor.
	if a, b := rune(8), rune(205); a*b == 1640 && Smush(a, b, RuleOppositePair) != 0 {
		t.Error("rule 4 matched a pair that only shares the product")
	}

	if got := Smush('(', ')', 0); got != 0 {
		t.Error("rule 4 fired while disabled")
	}
}

func TestSmushRule5BigX(t *testing.T) {
	cases := []struct {
		ch1, ch2 rune
		want     rune
	}{
		{'/', '\\', '|'},
		{'\\', '/', 'Y'},
		{'>', '<', 'X'},
		// The rule is not symmetric: these are not in the table.
		{'<', '>', 0},
		{'/', '/', 0},
	}

	for _, tc := range cases {
		if got := Smush(tc.ch1, tc.ch2, RuleBigX); got != tc.want {
			t.Errorf("Smush(%q, %q, rule 5) = %q, want %q",
				tc.ch1, tc.ch2, got, tc.want)
		}
	}

	if got := Smush('/', '\\', 0); got != 0 {
		t.Error("rule 5 fired while disabled")
	}
}

func TestSmushRule6HardblankUnreachable(t *testing.T) {
	// Rule 6 is meant to merge two hardblanks. libcaca guards rules 2 to 6
	// behind "both characters below U+0080", and a hardblank is held as
	// U+00A0, so the rule can never fire. The port keeps that, because it is
	// what makes hardblank columns survive smushing.
	if got := Smush(0xa0, 0xa0, RuleHardblank); got != 0 {
		t.Errorf("Smush(hardblank, hardblank, rule 6) = %q, want no smush", got)
	}
	if got := Smush(0xa0, 0xa0, allRules); got != 0 {
		t.Errorf("Smush(hardblank, hardblank, all rules) = %q, want no smush", got)
	}
}

func TestSmushRuleOrder(t *testing.T) {
	// Where several rules could apply, the lowest-numbered one wins.
	cases := []struct {
		ch1, ch2 rune
		rule     int
		want     rune
		why      string
	}{
		{'/', '/', allRules, '/', "rule 1 before rule 3"},
		{'/', '\\', allRules, '|', "same class, so rule 3 declines and rule 5 fires"},
		{'/', '\\', RuleBigX, '|', "rule 5 alone"},
		{'(', ')', allRules, '|', "same class, so rule 3 declines and rule 4 fires"},
		{'(', ')', RuleOppositePair, '|', "rule 4 alone"},
		{'_', '|', allRules, '|', "rule 2 before rule 3"},
	}

	for _, tc := range cases {
		if got := Smush(tc.ch1, tc.ch2, tc.rule); got != tc.want {
			t.Errorf("%s: Smush(%q, %q, %#x) = %q, want %q",
				tc.why, tc.ch1, tc.ch2, tc.rule, got, tc.want)
		}
	}
}

func TestSmushNonASCII(t *testing.T) {
	// Rules 2 to 6 are skipped entirely when either character is above
	// U+007F, so a box-drawing pair never smushes even under rule 3.
	for _, tc := range []struct{ a, b rune }{
		{'│', '─'}, {'_', '│'}, {0xa0, '|'}, {'|', 0xa0},
	} {
		if got := Smush(tc.a, tc.b, allRules); got != 0 {
			t.Errorf("Smush(%q, %q) = %q, want no smush", tc.a, tc.b, got)
		}
	}
}

func TestSmushEveryASCIIPairIsStable(t *testing.T) {
	// A returned character must be one of the two inputs or one of the four
	// substitutes the rules name. Anything else would be a table slip.
	allowed := map[rune]bool{'|': true, 'Y': true, 'X': true, 0xa0: true}

	for a := rune(0x20); a < 0x80; a++ {
		for b := rune(0x20); b < 0x80; b++ {
			got := Smush(a, b, allRules)
			if got == 0 || got == a || got == b || allowed[got] {
				continue
			}
			t.Fatalf("Smush(%q, %q) = %q, which is neither input nor a substitute",
				a, b, got)
		}
	}
}

func ExampleSmush() {
	fmt.Printf("%c %c %c\n",
		Smush('/', '\\', RuleBigX),
		Smush('\\', '/', RuleBigX),
		Smush('>', '<', RuleBigX))
	// Output: | Y X
}
