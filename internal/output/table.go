package output

import (
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	lgtable "github.com/charmbracelet/lipgloss/table"
)

// Table is a render-context–aware table builder.
//
// In TTY+color mode it uses lipgloss borders with styled headers.
// When piped (not TTY) or NO_COLOR is set, rows are tab-separated.
type Table struct {
	rc      *RenderContext
	headers []string
	rows    [][]string
}

// NewTable creates a Table using the given render context.
func NewTable(rc *RenderContext) *Table {
	return &Table{rc: rc}
}

// Headers sets the column headers.
func (t *Table) Headers(cols ...string) *Table {
	t.headers = cols
	return t
}

// Row appends a data row.
func (t *Table) Row(cols ...string) *Table {
	t.rows = append(t.rows, cols)
	return t
}

// Render writes the table to w.
//
// TTY + color: lipgloss styled table with muted borders and cyan headers.
// TTY + NO_COLOR: lipgloss table without color.
// Piped: tab-separated rows, no ANSI codes.
func (t *Table) Render(w io.Writer) {
	if !t.rc.IsTTY {
		t.renderTabSeparated(w)
		return
	}

	tableWidth := max(t.rc.Width-4, 40)

	tbl := lgtable.New().
		Border(lipgloss.NormalBorder()).
		Width(tableWidth).
		Headers(t.headers...)

	for _, row := range t.rows {
		tbl.Row(row...)
	}

	if t.rc.HasColor {
		headerStyle := lipgloss.NewStyle().Foreground(AccentColor).Bold(true)
		rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0"))
		tbl = tbl.
			BorderStyle(lipgloss.NewStyle().Foreground(MutedColor)).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == lgtable.HeaderRow {
					return headerStyle
				}
				return rowStyle
			})
	}

	rendered := tbl.Render()
	// Pitfall #4: gate explicit ANSI reset behind HasColor — never emit ANSI to piped output.
	if t.rc.HasColor {
		rendered += "\033[0m"
	}
	io.WriteString(w, rendered+"\n") //nolint:errcheck
}

// renderTabSeparated writes plain tab-separated output for piped consumption.
func (t *Table) renderTabSeparated(w io.Writer) {
	if len(t.headers) > 0 {
		io.WriteString(w, strings.Join(t.headers, "\t")+"\n") //nolint:errcheck
	}
	for _, row := range t.rows {
		io.WriteString(w, strings.Join(row, "\t")+"\n") //nolint:errcheck
	}
}
