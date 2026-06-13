package ui

import (
	"fmt"
	"io"
	"text/tabwriter"
)

type Table struct {
	w *tabwriter.Writer
}

func NewTable(out io.Writer) *Table {
	return &Table{w: tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)}
}

func (t *Table) Row(values ...any) {
	for i, value := range values {
		if i > 0 {
			fmt.Fprint(t.w, "\t")
		}
		fmt.Fprint(t.w, value)
	}
	fmt.Fprintln(t.w)
}

func (t *Table) Flush() error {
	return t.w.Flush()
}
