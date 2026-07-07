package main

import (
	"fmt"
	"os"
	"strings"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

var useColor bool

func init() {
	fi, err := os.Stderr.Stat()
	tty := err == nil && (fi.Mode()&os.ModeCharDevice) != 0
	useColor = tty && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
}

func paint(code, s string) string {
	if !useColor {
		return s
	}
	return code + s + ansiReset
}

// logStep writes a neutral progress line to stderr.
func logStep(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "  %s %s\n", paint(ansiDim, "·"), msg)
}

// logDone writes a success completion line to stderr.
func logDone(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "  %s %s\n", paint(ansiGreen+ansiBold, "✓"), msg)
}

// logWarn writes a warning line to stderr.
func logWarn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "  %s %s\n", paint(ansiYellow, "!"), msg)
}

// logSection writes a bold section header to stderr (used in verbose mode).
func logSection(title string) {
	fmt.Fprintf(os.Stderr, "\n%s\n", paint(ansiBold, "▸ "+title))
}

// fmtSource returns a colored "[source]" tag for a config key source.
func fmtSource(src string) string {
	switch {
	case strings.HasPrefix(src, "env:"):
		return paint(ansiCyan, "["+src+"]")
	case strings.HasPrefix(src, "flag:"):
		return paint(ansiGreen, "["+src+"]")
	case src == "default":
		return paint(ansiDim, "[default]")
	default: // "config file"
		return paint(ansiBold, "["+src+"]")
	}
}
