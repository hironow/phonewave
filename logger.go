package phonewave

import (
	"fmt"
	"os"
)

// ANSI color codes.
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
)

// LogOK prints a success message to stderr.
func LogOK(format string, args ...any) {
	fmt.Fprintf(os.Stderr, ColorGreen+"  ✓  "+ColorReset+format+"\n", args...)
}

// LogWarn prints a warning message to stderr.
func LogWarn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, ColorYellow+"  ⚠  "+ColorReset+format+"\n", args...)
}

// LogError prints an error message to stderr.
func LogError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, ColorRed+"  ✗  "+ColorReset+format+"\n", args...)
}

// LogInfo prints an informational message to stderr.
func LogInfo(format string, args ...any) {
	fmt.Fprintf(os.Stderr, ColorCyan+"  ⓘ  "+ColorReset+format+"\n", args...)
}
