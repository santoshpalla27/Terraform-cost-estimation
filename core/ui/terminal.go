// Package ui - Terminal user interface
// Rich CLI output with progress bars, tables, and colors.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Colors for terminal output
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
)

// Writer is the UI output destination
type Writer struct {
	out       io.Writer
	noColor   bool
	verbosity int
}

// NewWriter creates a UI writer
func NewWriter(out io.Writer, noColor bool) *Writer {
	if out == nil {
		out = os.Stdout
	}
	return &Writer{
		out:       out,
		noColor:   noColor,
		verbosity: 1,
	}
}

// SetVerbosity sets output verbosity (0=quiet, 1=normal, 2=verbose)
func (w *Writer) SetVerbosity(level int) {
	w.verbosity = level
}

// color applies color if enabled
func (w *Writer) color(c, text string) string {
	if w.noColor {
		return text
	}
	return c + text + Reset
}

// Print writes a line
func (w *Writer) Print(format string, args ...interface{}) {
	fmt.Fprintf(w.out, format, args...)
}

// Println writes a line with newline
func (w *Writer) Println(format string, args ...interface{}) {
	fmt.Fprintf(w.out, format+"\n", args...)
}

// Header prints a section header
func (w *Writer) Header(title string) {
	w.Println("")
	w.Println(w.color(Bold+Cyan, "━━━ "+title+" ━━━"))
	w.Println("")
}

// SubHeader prints a subsection header
func (w *Writer) SubHeader(title string) {
	w.Println(w.color(Bold, "▸ "+title))
}

// Success prints a success message
func (w *Writer) Success(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	w.Println(w.color(Green, "✓ ")+msg)
}

// Warning prints a warning
func (w *Writer) Warning(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	w.Println(w.color(Yellow, "⚠ ")+msg)
}

// Error prints an error
func (w *Writer) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	w.Println(w.color(Red, "✗ ")+msg)
}

// Info prints an info message
func (w *Writer) Info(format string, args ...interface{}) {
	if w.verbosity < 1 {
		return
	}
	msg := fmt.Sprintf(format, args...)
	w.Println(w.color(Blue, "ℹ ")+msg)
}

// Debug prints a debug message
func (w *Writer) Debug(format string, args ...interface{}) {
	if w.verbosity < 2 {
		return
	}
	msg := fmt.Sprintf(format, args...)
	w.Println(w.color(Dim, "  "+msg))
}

// ProgressBar renders a progress bar
type ProgressBar struct {
	w         *Writer
	total     int
	current   int
	width     int
	label     string
	startTime time.Time
}

// NewProgressBar creates a progress bar
func (w *Writer) NewProgressBar(total int, label string) *ProgressBar {
	return &ProgressBar{
		w:         w,
		total:     total,
		width:     40,
		label:     label,
		startTime: time.Now(),
	}
}

// Update updates the progress bar
func (p *ProgressBar) Update(current int) {
	p.current = current
	p.render()
}

// Increment increments the progress bar
func (p *ProgressBar) Increment() {
	p.current++
	p.render()
}

func (p *ProgressBar) render() {
	if p.total == 0 {
		return
	}

	percent := float64(p.current) / float64(p.total)
	filled := int(percent * float64(p.width))
	
	bar := strings.Repeat("█", filled) + strings.Repeat("░", p.width-filled)
	
	// Calculate ETA
	elapsed := time.Since(p.startTime)
	eta := ""
	if p.current > 0 {
		remaining := time.Duration(float64(elapsed) / float64(p.current) * float64(p.total-p.current))
		eta = fmt.Sprintf(" ETA: %s", formatDuration(remaining))
	}

	fmt.Fprintf(p.w.out, "\r%s [%s] %3.0f%% (%d/%d)%s", 
		p.label, bar, percent*100, p.current, p.total, eta)
}

// Done completes the progress bar
func (p *ProgressBar) Done() {
	fmt.Fprintln(p.w.out)
}

// Table renders a table
type Table struct {
	w       *Writer
	headers []string
	rows    [][]string
	widths  []int
}

// NewTable creates a table
func (w *Writer) NewTable(headers ...string) *Table {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	return &Table{
		w:       w,
		headers: headers,
		rows:    [][]string{},
		widths:  widths,
	}
}

// AddRow adds a row to the table
func (t *Table) AddRow(cells ...string) {
	// Pad or truncate cells to match header count
	row := make([]string, len(t.headers))
	for i := range row {
		if i < len(cells) {
			row[i] = cells[i]
		}
		if len(row[i]) > t.widths[i] {
			t.widths[i] = len(row[i])
		}
	}
	t.rows = append(t.rows, row)
}

// Render prints the table
func (t *Table) Render() {
	// Build format string
	format := ""
	for i, w := range t.widths {
		if i > 0 {
			format += " │ "
		}
		format += fmt.Sprintf("%%-%ds", w)
	}
	format += "\n"

	// Header
	headerArgs := make([]interface{}, len(t.headers))
	for i, h := range t.headers {
		headerArgs[i] = h
	}
	t.w.Print(t.w.color(Bold, fmt.Sprintf(format, headerArgs...)))

	// Separator
	sep := ""
	for i, w := range t.widths {
		if i > 0 {
			sep += "─┼─"
		}
		sep += strings.Repeat("─", w)
	}
	t.w.Println(sep)

	// Rows
	for _, row := range t.rows {
		args := make([]interface{}, len(row))
		for i, cell := range row {
			args[i] = cell
		}
		t.w.Print(format, args...)
	}
}

// CostSummary renders a cost summary
type CostSummary struct {
	w           *Writer
	TotalMonthly string
	TotalHourly  string
	Confidence   float64
	Resources    int
	Warnings     int
}

// NewCostSummary creates a cost summary
func (w *Writer) NewCostSummary() *CostSummary {
	return &CostSummary{w: w}
}

// Render prints the cost summary
func (s *CostSummary) Render() {
	s.w.Header("Cost Estimation Summary")

	// Main cost box
	s.w.Println(s.w.color(Bold, "╭─────────────────────────────────────╮"))
	s.w.Println(s.w.color(Bold, "│")+s.w.color(Green, fmt.Sprintf("  Monthly Cost: %-20s", s.TotalMonthly))+s.w.color(Bold, "│"))
	s.w.Println(s.w.color(Bold, "│")+s.w.color(Dim, fmt.Sprintf("  Hourly Cost:  %-20s", s.TotalHourly))+s.w.color(Bold, "│"))
	s.w.Println(s.w.color(Bold, "╰─────────────────────────────────────╯"))

	s.w.Println("")

	// Confidence indicator
	confColor := Green
	confIcon := "●"
	if s.Confidence < 0.8 {
		confColor = Yellow
		confIcon = "◐"
	}
	if s.Confidence < 0.5 {
		confColor = Red
		confIcon = "○"
	}
	s.w.Println(s.w.color(confColor, fmt.Sprintf("%s Confidence: %.0f%%", confIcon, s.Confidence*100)))

	// Stats
	s.w.Println(s.w.color(Dim, fmt.Sprintf("  Resources: %d", s.Resources)))
	if s.Warnings > 0 {
		s.w.Warning("%d warnings", s.Warnings)
	}
}

// Spinner shows a loading spinner
type Spinner struct {
	w       *Writer
	label   string
	frames  []string
	current int
	stop    chan struct{}
	done    chan struct{}
}

// NewSpinner creates a spinner
func (w *Writer) NewSpinner(label string) *Spinner {
	return &Spinner{
		w:      w,
		label:  label,
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

// Start starts the spinner
func (s *Spinner) Start() {
	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				close(s.done)
				return
			case <-ticker.C:
				s.current = (s.current + 1) % len(s.frames)
				fmt.Fprintf(s.w.out, "\r%s %s", s.w.color(Cyan, s.frames[s.current]), s.label)
			}
		}
	}()
}

// Stop stops the spinner
func (s *Spinner) Stop(success bool) {
	close(s.stop)
	<-s.done
	
	icon := s.w.color(Green, "✓")
	if !success {
		icon = s.w.color(Red, "✗")
	}
	fmt.Fprintf(s.w.out, "\r%s %s\n", icon, s.label)
}

// ResourceDiff shows resource cost diff
type ResourceDiff struct {
	w           *Writer
	Added       []DiffItem
	Removed     []DiffItem
	Changed     []DiffItem
	TotalChange string
	IsIncrease  bool
}

// DiffItem is a single diff item
type DiffItem struct {
	Address    string
	OldCost    string
	NewCost    string
	Change     string
	IsIncrease bool
}

// NewResourceDiff creates a diff view
func (w *Writer) NewResourceDiff() *ResourceDiff {
	return &ResourceDiff{w: w}
}

// Render prints the diff
func (d *ResourceDiff) Render() {
	d.w.Header("Cost Changes")

	// Added
	if len(d.Added) > 0 {
		d.w.SubHeader(fmt.Sprintf("Added (%d)", len(d.Added)))
		for _, item := range d.Added {
			d.w.Println(d.w.color(Green, "+ ")+"%s: %s", item.Address, item.NewCost)
		}
		d.w.Println("")
	}

	// Removed
	if len(d.Removed) > 0 {
		d.w.SubHeader(fmt.Sprintf("Removed (%d)", len(d.Removed)))
		for _, item := range d.Removed {
			d.w.Println(d.w.color(Red, "- ")+"%s: %s", item.Address, item.OldCost)
		}
		d.w.Println("")
	}

	// Changed
	if len(d.Changed) > 0 {
		d.w.SubHeader(fmt.Sprintf("Changed (%d)", len(d.Changed)))
		for _, item := range d.Changed {
			arrow := d.w.color(Yellow, "→")
			change := item.Change
			if item.IsIncrease {
				change = d.w.color(Red, "+"+change)
			} else {
				change = d.w.color(Green, change)
			}
			d.w.Println("  %s: %s %s %s (%s)", item.Address, item.OldCost, arrow, item.NewCost, change)
		}
		d.w.Println("")
	}

	// Total
	d.w.Println(strings.Repeat("─", 40))
	changeColor := Green
	changePrefix := ""
	if d.IsIncrease {
		changeColor = Red
		changePrefix = "+"
	}
	d.w.Println(d.w.color(Bold, "Total Change: ")+d.w.color(changeColor, changePrefix+d.TotalChange))
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "< 1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
}
