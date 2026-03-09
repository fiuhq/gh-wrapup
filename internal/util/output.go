package util

import (
	"fmt"
	"os"
)

// ANSI colour codes — automatically disabled when not writing to a terminal.
const (
	colReset  = "\033[0m"
	colGreen  = "\033[32m"
	colRed    = "\033[31m"
	colYellow = "\033[33m"
	colGrey   = "\033[90m"
)

// isTTY returns true when stdout is a real terminal (colours enabled).
func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func green(s string) string {
	if !isTTY() {
		return s
	}
	return colGreen + s + colReset
}

func red(s string) string {
	if !isTTY() {
		return s
	}
	return colRed + s + colReset
}

func yellow(s string) string {
	if !isTTY() {
		return s
	}
	return colYellow + s + colReset
}

func grey(s string) string {
	if !isTTY() {
		return s
	}
	return colGrey + s + colReset
}

// Success prints a green checkmark line to stdout.
//
//	✓ Issue #42 created: https://github.com/org/repo/issues/42
func Success(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(green("✓") + " " + msg)
}

// Failure prints a red X line to stderr.
//
//	✗ Failed to create issue: <error>
func Failure(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, red("✗")+" "+msg)
}

// Info prints a neutral informational line to stdout.
//
//	· Found issue #42: My title
func Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(grey("·") + " " + msg)
}

// Reused prints a yellow "reused" indicator (for upsert idempotency output).
//
//	~ Issue #42 (existing): https://...
func Reused(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(yellow("~") + " " + msg)
}

// Tree prints a tree-indented child line, used to show linked items.
//
//	  └─ Closes #42
func Tree(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println("  " + grey("└─") + " " + msg)
}
