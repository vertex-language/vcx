package cli

import (
	"flag"
	"fmt"
	"io"
)

// cmdLayout prints the computed memory layout (sizes, alignment, and member offsets) for every class in a file.
func cmdLayout(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("layout", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var pp ppFlags
	pp.register(fs)

	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "v++ layout: one file at a time")
		return exitUsage
	}

	c, err := pp.compiler()
	if err != nil {
		fmt.Fprintln(stderr, "v++:", err)
		return exitUsage
	}
	in, err := input(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "v++:", err)
		return exitUsage
	}

	records, diags, err := c.Layout(in)
	if err != nil {
		fmt.Fprintln(stderr, "v++:", err)
		return exitUsage
	}
	hadErrors := printDiags(stderr, diags)

	for i, r := range records {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprintf(stdout, "%s %s size=%d align=%d\n", r.Kind, r.Name, r.Size, r.Align)
		for _, b := range r.Bases {
			kind := "base"
			if b.Virtual {
				kind = "virtual base"
			}
			fmt.Fprintf(stdout, "%10d  (%s %s)\n", b.Offset, kind, b.Name)
		}
		for _, f := range r.Fields {
			if f.BitField {
				fmt.Fprintf(stdout, "%10d  %s : %d\n", f.Offset, f.Name, f.Width)
				continue
			}
			fmt.Fprintf(stdout, "%10d  %s\n", f.Offset, f.Name)
		}

		// The tables, after the members, the way cl prints them. A slot
		// names the class whose definition it holds, not the class that
		// gave the slot its number -- which is the whole point of a slot.
		for _, vt := range r.VTables {
			label := r.Name + "::vftable"
			if vt.Base != "" && len(r.VTables) > 1 {
				label += "@" + vt.Base
			}
			fmt.Fprintf(stdout, "%s (at %d)\n", label, vt.Offset)
			for slot, sl := range vt.Slots {
				pure := ""
				if sl.Pure {
					pure = " = 0"
				}
				adjust := ""
				if sl.Adjust != 0 {
					adjust = fmt.Sprintf("  [this adjustor: %d]", sl.Adjust)
				}
				fmt.Fprintf(stdout, "%10d  %s::%s%s%s\n", slot, sl.Definer, sl.Name, pure, adjust)
			}
		}

		for _, vb := range r.VBTables {
			label := r.Name + "::vbtable"
			if vb.Base != "" {
				label += "@" + vb.Base
			}
			fmt.Fprintf(stdout, "%s (at %d)\n", label, vb.Offset)
			for entry, off := range vb.Entries {
				fmt.Fprintf(stdout, "%10d  %d\n", entry, off)
			}
		}
	}

	if hadErrors {
		return exitDiags
	}
	return exitOK
}
