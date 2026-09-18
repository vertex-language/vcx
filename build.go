package vcx

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Build compiles every source input to an object and, unless asked to
// stop there, links the objects -- and any object given as an input --
// into an executable. An offload unit's object carries its device image;
// a program with one links vcx's own runtime for the language too, over
// the vendor's driver, unless a toolkit's runtime is named with -l.
func (c *Compiler) Build(params BuildParams) error {
	if len(params.Inputs) == 0 {
		return fmt.Errorf("no input files")
	}
	var objects []Input
	offload := map[Language]bool{}
	for _, in := range params.Inputs {
		if !in.isSource() {
			// An object built earlier: it needs the runtime if it
			// registers a device image, which its symbol table says.
			if data, err := in.bytes(); err == nil {
				if bytes.Contains(data, []byte("__cudaRegisterFatBinary")) {
					offload[LangCUDA] = true
				}
				if bytes.Contains(data, []byte("__hipRegisterFatBinary")) {
					offload[LangHIP] = true
				}
			}
			objects = append(objects, in)
			continue
		}
		obj, diags, err := c.Object(in)
		if err != nil {
			return err
		}
		if HasErrors(diags) {
			return &DiagnosticError{Diagnostics: diags}
		}
		if obj == nil {
			return fmt.Errorf("no object produced for %s", in.Name)
		}
		if lang := c.language(in); lang != LangCXX && !c.DeviceOnly {
			offload[lang] = true
		}
		objects = append(objects, ObjectBytes(moduleName(in)+c.objectExt(in), obj))
	}

	if params.CompileOnly || c.DeviceOnly {
		return c.writeObjects(params, objects)
	}

	// The runtime of each offload language present, as an object of its
	// own, after the program's objects.
	for _, lang := range []Language{LangCUDA, LangHIP} {
		if !offload[lang] {
			continue
		}
		rt, err := c.runtimeObject(lang)
		if err != nil {
			return err
		}
		objects = append(objects, rt)
	}
	out := params.Output
	if out == "" {
		out = c.defaultExecutable()
	}
	return c.Link(LinkParams{Objects: objects, Output: out, Libs: params.Libs, LibDirs: params.LibDirs})
}

// writeObjects writes each object where -c and -o say: one output for
// one input, a directory or the working directory for several.
func (c *Compiler) writeObjects(params BuildParams, objects []Input) error {
	for _, o := range objects {
		if o.Data == nil {
			continue // an object given by path is already on disk
		}
		out := o.Name
		switch {
		case params.Output == "":
		case len(objects) == 1:
			out = params.Output
		default:
			out = filepath.Join(params.Output, o.Name)
		}
		if err := os.WriteFile(out, o.Data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// defaultExecutable is the output when -o names none: a.exe on Windows,
// a.out elsewhere, as every driver has it.
func (c *Compiler) defaultExecutable() string {
	if t, err := c.target(); err == nil && t.OS == "windows" {
		return "a.exe"
	}
	return "a.out"
}

// The offload runtimes vcx ships: the cudart and hip entry points a
// compiled unit calls, over the vendor's driver, compiled here for the
// host at link time.
//
//go:embed runtime/cuda/vcx_cudart.cpp runtime/hip/vcx_hiprt.cpp
var runtimeFS embed.FS

// runtimeObject compiles the language's runtime for the host.
func (c *Compiler) runtimeObject(lang Language) (Input, error) {
	var path string
	switch lang {
	case LangCUDA:
		path = "runtime/cuda/vcx_cudart.cpp"
	case LangHIP:
		path = "runtime/hip/vcx_hiprt.cpp"
	default:
		return Input{}, fmt.Errorf("no runtime for %s", lang)
	}
	src, err := runtimeFS.ReadFile(path)
	if err != nil {
		return Input{}, fmt.Errorf("vcx: embedded runtime missing: %w", err)
	}
	// The runtime is a host-only unit of its own language: it reads the
	// same headers the program did, and defines no kernel.
	rc := *c
	rc.Language = lang
	rc.HostOnly, rc.DeviceOnly = true, false
	in := Text(filepath.Base(path), src)
	obj, diags, err := rc.Object(in)
	if err != nil {
		return Input{}, err
	}
	if HasErrors(diags) {
		return Input{}, &DiagnosticError{Diagnostics: diags}
	}
	return ObjectBytes(strings.TrimSuffix(filepath.Base(path), ".cpp")+".o", obj), nil
}

// Run builds the inputs into an executable in a temporary directory,
// runs it, and is what it wrote to standard output. A program that
// exits non-zero is an error carrying its output and status.
func (c *Compiler) Run(path string, args ...string) ([]byte, error) {
	dir, err := os.MkdirTemp("", "vcx-run-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	exe := filepath.Join(dir, "a.out")
	if runtime.GOOS == "windows" {
		exe = filepath.Join(dir, "a.exe")
	}
	if err := c.Build(BuildParams{Output: exe, Inputs: []Input{File(path)}}); err != nil {
		return nil, err
	}
	cmd := exec.Command(exe, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return stdout.Bytes(), &RunError{Err: err, Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	}
	return stdout.Bytes(), nil
}

// A RunError is a program that did not exit cleanly.
type RunError struct {
	Err    error
	Stdout []byte
	Stderr []byte
}

func (e *RunError) Error() string {
	msg := e.Err.Error()
	if len(e.Stderr) > 0 {
		msg += "\n" + strings.TrimRight(string(e.Stderr), "\n")
	}
	return msg
}

func (e *RunError) Unwrap() error { return e.Err }

// objectExt is the extension of what Object produces for the input: an
// object file, or a device image for an offload unit's device pass.
func (c *Compiler) objectExt(in Input) string {
	p, err := c.passFor(in)
	if err != nil || !p.device {
		if t, err := c.target(); err == nil && t.Container == ContainerPE {
			return ".obj"
		}
		return ".o"
	}
	if p.tgt.Container == ContainerPTX {
		return ".ptx"
	}
	return ".hsaco"
}
