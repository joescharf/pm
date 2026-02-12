package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

// UI provides colored output and respects verbose/dry-run modes.
type UI struct {
	Verbose bool
	DryRun  bool
	Out     io.Writer
	ErrOut  io.Writer
}

// New creates a UI with default stdout/stderr writers.
func New() *UI {
	return &UI{
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
}

var (
	infoPrefix    = color.New(color.FgHiBlue).Sprint("i")
	successPrefix = color.New(color.FgHiGreen).Sprint("\u2713")
	warningPrefix = color.New(color.FgHiYellow).Sprint("\u26a0")
	errorPrefix   = color.New(color.FgHiRed).Sprint("\u2717")
	verbosePrefix = color.New(color.FgHiBlue).Sprint("  \u2192")
	cyan          = color.New(color.FgHiCyan).SprintFunc()
	green         = color.New(color.FgHiGreen).SprintFunc()
	yellow        = color.New(color.FgHiYellow).SprintFunc()
	red           = color.New(color.FgHiRed).SprintFunc()
)

// Cyan returns a cyan-colored string.
func Cyan(s string) string { return cyan(s) }

// Green returns a green-colored string.
func Green(s string) string { return green(s) }

// Yellow returns a yellow-colored string.
func Yellow(s string) string { return yellow(s) }

// Red returns a red-colored string.
func Red(s string) string { return red(s) }

// StatusColor returns the string colored by issue status.
func StatusColor(status string) string {
	switch strings.ToLower(status) {
	case "open":
		return green(status)
	case "in_progress":
		return yellow(status)
	case "done", "completed":
		return cyan(status)
	case "closed":
		return red(status)
	default:
		return status
	}
}

// HealthColor returns the string colored by health score.
func HealthColor(score int) string {
	s := fmt.Sprintf("%d", score)
	switch {
	case score >= 80:
		return green(s)
	case score >= 50:
		return yellow(s)
	default:
		return red(s)
	}
}

func (u *UI) Info(format string, a ...any) {
	fmt.Fprintf(u.Out, "%s %s\n", infoPrefix, fmt.Sprintf(format, a...))
}

func (u *UI) Success(format string, a ...any) {
	fmt.Fprintf(u.Out, "%s %s\n", successPrefix, fmt.Sprintf(format, a...))
}

func (u *UI) Warning(format string, a ...any) {
	fmt.Fprintf(u.ErrOut, "%s %s\n", warningPrefix, fmt.Sprintf(format, a...))
}

func (u *UI) Error(format string, a ...any) {
	fmt.Fprintf(u.ErrOut, "%s %s\n", errorPrefix, fmt.Sprintf(format, a...))
}

func (u *UI) VerboseLog(format string, a ...any) {
	if u.Verbose {
		fmt.Fprintf(u.Out, "%s %s\n", verbosePrefix, fmt.Sprintf(format, a...))
	}
}

func (u *UI) DryRunMsg(format string, a ...any) {
	if u.DryRun {
		u.Warning("[DRY-RUN] "+format, a...)
	}
}

// Table creates a new tablewriter configured with consistent styling.
func (u *UI) Table(headers []string) *tablewriter.Table {
	table := tablewriter.NewTable(u.Out,
		tablewriter.WithHeaderAlignment(tw.AlignLeft),
		tablewriter.WithRowAlignment(tw.AlignLeft),
		tablewriter.WithRendition(tw.Rendition{
			Borders: tw.BorderNone,
			Settings: tw.Settings{
				Lines:      tw.LinesNone,
				Separators: tw.SeparatorsNone,
			},
		}),
		tablewriter.WithPadding(tw.Padding{Left: "", Right: "  "}),
	)
	table.Header(headers)
	return table
}
