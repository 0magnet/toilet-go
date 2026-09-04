//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package main

// terminalWidth has no way to ask on this platform, so -t leaves the width
// alone, exactly as the original does when built without TIOCGWINSZ.
func terminalWidth() (int, bool) { return 0, false }
