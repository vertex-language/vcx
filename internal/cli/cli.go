package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vertex-language/air/bitcode"
	airtext "github.com/vertex-language/air/text"

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
    v++ build  [flags] [files...]   compile and link an executable; -c stops at objects,
                                    --emit vir|ptx|hsaco one rung short; a .metal
                                    file builds to a .metallib (--emit air|ll for
                                    its bitcode or text)
    v++ run    [flags] [file] [-- args...]
                                    build to a temporary path and run it
    v++ [flags] files...            the driver spelling: a build, with the
                                    flags and files in any order, as nvcc,
                                    hipcc and clang++ take them
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

Offload flags (a .cu is CUDA, a .hip is HIP, as nvcc and hipcc have it,
and a .metal is Metal):
    -x cuda|hip|metal
                    the language, whatever the extension says
    --offload-arch  the device: sm_75, gfx942, apple8, ... (default sm_52
                    for CUDA, apple7 for Metal)
    -std=metal3.1   the MSL version of a .metal file (default metal3.0)
    -mmacosx-version-min=14.0
                    the oldest macOS a .metallib loads on (default 13.0)
    -fno-fast-math  precise float math in a .metal file
    --cuda-device-only / --cuda-host-only
                    one pass of an offload unit rather than both

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
	if !isCommand(verb) {
		// The driver spelling: v++ [flags] files... is a build, read the
		// way nvcc and clang++ read their command lines.
		return cmdBuild(args, stdout, stderr)
	}
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
	// Flags after the files too, as every driver allows.
	args, _ = driverArgs(args)
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var pp ppFlags
	pp.register(fs)
	outPath := fs.String("o", "", "output file (default a.exe or a.out, or the object's name with -c)")
	emit := fs.String("emit", "", "what to produce: vir, obj, ptx, hsaco; for a .metal file metallib, air or ll (obj implies -c)")
	compileOnly := fs.Bool("c", false, "compile to objects, do not link")
	var libs, libDirs stringList
	fs.Var(&libs, "l", "link a library (repeatable)")
	var frameworks stringList
	fs.Var(&frameworks, "framework", "link an Apple framework (repeatable)")
	fs.Var(&libDirs, "L", "add a library search directory (repeatable)")
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

	// `--emit ptx` and `--emit hsaco` are the device pass alone, written
	// as the image the driver loads.
	switch *emit {
	case "ptx", "hsaco":
		c.DeviceOnly, c.HostOnly = true, false
		*compileOnly = true
	case "obj":
		*compileOnly = true
	case "air", "ll":
		// One .metal file's module, as bitcode or as text: what xcrun's
		// metal -c and -S write.
		return emitAIR(c, inputs, *emit, *outPath, stdout, stderr)
	case "vir", "", "metallib":
	default:
		fmt.Fprintf(stderr, "v++: --emit %s: not vir, obj, ptx, hsaco, metallib, air or ll\n", *emit)
		return exitUsage
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
		Output:      *outPath,
		Inputs:      inputs,
		Libs:        libs,
		Frameworks:  frameworks,
		LibDirs:     libDirs,
		CompileOnly: *compileOnly,
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

	out, err := c.Run(fs.Arg(0), fs.Args()[1:]...)
	stdout.Write(out)
	if err != nil {
		var run *vcx.RunError
		if errors.As(err, &run) {
			stderr.Write(run.Stderr)
			var exit *exec.ExitError
			if errors.As(run.Err, &exit) {
				return exit.ExitCode()
			}
		}
		fmt.Fprintln(stderr, "v++:", err)
		return exitDiags
	}
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

// emitAIR writes each .metal input's AIR module: bitcode for air, the
// text form for ll. One input may name its output with -o; otherwise
// each goes next to its source's name, or to standard output for ll.
func emitAIR(c *vcx.Compiler, inputs []vcx.Input, kind, out string, stdout, stderr io.Writer) int {
	if out != "" && len(inputs) != 1 {
		fmt.Fprintf(stderr, "v++: -o with --emit %s names one output, for one input\n", kind)
		return exitUsage
	}
	for _, in := range inputs {
		m, diags, err := c.AIR(in)
		if printDiags(stderr, diags) {
			return exitDiags
		}
		if err != nil {
			fmt.Fprintln(stderr, "v++:", err)
			return exitDiags
		}
		var data []byte
		if kind == "air" {
			data, err = bitcode.Encode(m)
		} else {
			var s string
			s, err = airtext.Print(m)
			data = []byte(s)
		}
		if err != nil {
			fmt.Fprintln(stderr, "v++:", err)
			return exitDiags
		}
		path := out
		if path == "" {
			if kind == "ll" {
				stdout.Write(data)
				continue
			}
			path = strings.TrimSuffix(filepath.Base(in.Name), filepath.Ext(in.Name)) + ".air"
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			fmt.Fprintln(stderr, "v++:", err)
			return exitDiags
		}
	}
	return exitOK
}
