package setup

import (
	"fmt"
	"io"
	"text/tabwriter"
)

// columnGap is the space between table columns.
const columnGap = 2

// printTable shows the lights zigbeat may drive, numbered for the question,
// and why the other lights and switches are left out.
func printTable(w io.Writer, devices int, offered []light, fixture map[string]string, marked map[string]bool, refused []refusal, others int) {
	fmt.Fprintf(w, "zigbeat setup: %d devices, %d lights offered\n\n", devices, len(offered))
	tw := tabwriter.NewWriter(w, 0, 0, columnGap, ' ', 0)
	fmt.Fprintln(tw, "  #\tlight\trole\tfixture\tmodel")
	for i, l := range offered {
		mark := ""
		if marked[l.name] {
			mark = " *"
		}
		fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\t%s%s\n", i+1, l.name, l.role, fixture[l.name], l.model, mark)
	}
	tw.Flush()
	if len(refused) > 0 {
		fmt.Fprintln(w, "\nnot offered:")
		tw = tabwriter.NewWriter(w, 0, 0, columnGap, ' ', 0)
		for _, r := range refused {
			fmt.Fprintf(tw, "  %s\t%s\n", r.name, r.why)
		}
		tw.Flush()
	}
	if others > 0 {
		fmt.Fprintf(w, "not listed: devices without a light, such as sensors and remotes (%d)\n", others)
	}
	fmt.Fprintln(w)
}
