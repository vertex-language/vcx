package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/vertex-language/ir/text"

	"github.com/vertex-language/vcx"
)

const (
	exitOK    = 0
	exitDiags = 1
	exitUsage = 2
)

const usage = `v++ — the Vertex C++ Compiler

Usage:
    v++ build  [flags] [files...]   compile to an object; --emit vir stops one rung short
    v++ run    [flags] [file]       build to a temporary path and run it
    v++ check  [flags] [files...]   preprocess, parse, analyze; print diagnostics
    v++ ast    [flags] [file]       parse and dump the syntax tree
    v++ layout [flags] [file]       print the computed layout of every class
    v++ symbols [flags] [file]      print the object-file name of every definition
    v++ tokens [flags] [file]       dump the token stream
    v++ env    [flags]              print resolved configuration

Common flags:
    -target T       target to compile for (default: this host)
    -std S          language standard: c++20, c++23 (default), c++26
    -I dir          add an include search directory (repeatable, in order)
    -D name[=val]   define a macro (repeatable)
    -U name         undefine a macro (repeatable)
    -freestanding   freestanding environment (no standard library)

Flags for ast:
    -skip-bodies    skip function bodies (fast structural pass)
    -comments       retain comments on the tree
`

// Run executes a v++ command line invocation and returns the exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}

	verb, rest := args[0], args[1:]
	switch verb {
	case "tokens":
		return cmdTokens(rest, stdout, stderr)
	case "ast":
		return cmdAST(rest, stdout, stderr)
	case "layout":
		return cmdLayout(rest, stdout, stderr)
	case "symbols":
		return cmdSymbols(rest, stdout, stderr)
	case "check":
		return cmdCheck(rest, stdout, stderr)
	case "build":
		return cmdBuild(rest, stdout, stderr)
	case "run":
		return cmdRun(rest, stdout, stderr)
	case "env":
		return cmdEnv(rest, stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	default:
		fmt.Fprintf(stderr, "v++: unknown command %q\nRun 'v++ help' for usage.\n", verb)
		return exitUsage
	}
}

func cmdCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var pp ppFlags
	pp.register(fs)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	c, err := pp.compiler()
	if err != nil {
		fmt.Fprintln(stderr, "v++:", err)
		return exitUsage
	}

	hadErrors := false
	for _, arg := range fs.Args() {
		in, err := input(arg)
		if err != nil {
			fmt.Fprintln(stderr, "v++:", err)
			return exitUsage
		}
		diags, err := c.Check(in)
		if err != nil {
			fmt.Fprintln(stderr, "v++:", err)
			return exitUsage
		}
		if printDiags(stderr, diags) {
			hadErrors = true
		}
	}

	if hadErrors {
		return exitDiags
	}
	return exitOK
}

func cmdBuild(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var pp ppFlags
	pp.register(fs)
	outPath := fs.String("o", "a.out", "output file")
	emit := fs.String("emit", "obj", "what to produce: vir, obj")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	c, err := pp.compiler()
	if err != nil {
		fmt.Fprintln(stderr, "v++:", err)
		return exitUsage
	}

	var inputs []vcx.Input
	for _, arg := range fs.Args() {
		in, err := input(arg)
		if err != nil {
			fmt.Fprintln(stderr, "v++:", err)
			return exitUsage
		}
		inputs = append(inputs, in)
	}

	// `--emit vir` stops one rung short and prints the module. It is the
	// flag wanted most when something is wrong, so it prints whatever was
	// built even on a file that drew diagnostics.
	if *emit == "vir" {
		for _, in := range inputs {
			mod, diags, err := c.IR(in)
			if err != nil {
				fmt.Fprintln(stderr, "v++:", err)
				return exitUsage
			}
			hadErrors := printDiags(stderr, diags)
			if mod != nil {
				text.Print(stdout, mod)
			}
			if hadErrors {
				return exitDiags
			}
		}
		return exitOK
	}

	err = c.Build(vcx.BuildParams{
		Output: *outPath,
		Inputs: inputs,
	})
	if err != nil {
		fmt.Fprintln(stderr, "v++:", err)
		return exitDiags
	}
	return exitOK
}

func cmdRun(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var pp ppFlags
	pp.register(fs)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "v++ run: no input file provided")
		return exitUsage
	}

	c, err := pp.compiler()
	if err != nil {
		fmt.Fprintln(stderr, "v++:", err)
		return exitUsage
	}

	out, err := c.Run(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, "v++:", err)
		return exitDiags
	}
	stdout.Write(out)
	return exitOK
}

func cmdEnv(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("env", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var pp ppFlags
	pp.register(fs)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	c, err := pp.compiler()
	if err != nil {
		fmt.Fprintln(stderr, "v++:", err)
		return exitUsage
	}
	target := vcx.DefaultTarget()
	if c.Target != "" {
		if t, err := vcx.TargetByName(c.Target); err == nil {
			target = t
		}
	}
	fmt.Fprintf(stdout, "Target:     %s (%s, %s, %s)\n", target.Name, target.Container, target.ABI, target.Dialect)
	fmt.Fprintf(stdout, "DefaultStd: C++23\n")
	fmt.Fprintf(stdout, "Includes:\n")
	for _, inc := range c.IncludeDirs {
		fmt.Fprintf(stdout, "    %s\n", inc)
	}
	for _, sys := range c.SystemIncludes() {
		fmt.Fprintf(stdout, "    %s  (system)\n", sys.Name)
	}
	return exitOK
}
