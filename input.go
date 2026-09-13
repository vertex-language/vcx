package vcx

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/vertex-language/vcx/preprocessor"
	"github.com/vertex-language/vcx/token"
)

// An Input is one file or memory buffer to compile or link.
type Input struct {
	Name string
	Data []byte
	FS   fs.FS
}

// File is an input read from disk.
func File(path string) Input { return Input{Name: path} }

// Text is source already in memory.
func Text(name string, data []byte) Input { return Input{Name: name, Data: data} }

// ObjectBytes is an object file already in memory.
func ObjectBytes(name string, data []byte) Input { return Input{Name: name, Data: data} }

func (in Input) name() string {
	if in.Name == "" || in.Name == "-" {
		return "<stdin>"
	}
	return in.Name
}

// isSource reports whether this input is a C++ source file.
func (in Input) isSource() bool {
	ext := strings.ToLower(filepath.Ext(in.Name))
	switch ext {
	case ".cpp", ".cc", ".cxx", ".c++", ".cp", ".c", ".cppm", ".ixx", ".ccm", ".cxxm", ".c++m", ".ii":
		return true
	}
	return in.name() == "<stdin>"
}

// isInterfaceUnit reports whether this input is a module interface unit (.cppm, .ixx, etc.).
func (in Input) isInterfaceUnit() bool {
	ext := strings.ToLower(filepath.Ext(in.Name))
	switch ext {
	case ".cppm", ".ixx", ".ccm", ".cxxm", ".c++m":
		return true
	}
	return false
}

// isPreprocessed reports whether this input is already preprocessed source (.ii).
func (in Input) isPreprocessed() bool {
	return strings.ToLower(filepath.Ext(in.Name)) == ".ii"
}

func (in Input) bytes() ([]byte, error) {
	if in.Data != nil {
		return in.Data, nil
	}
	if in.name() == "<stdin>" {
		return nil, &fs.PathError{Op: "open", Path: "<stdin>", Err: fs.ErrInvalid}
	}
	return os.ReadFile(in.Name)
}

func (in Input) load() (*token.File, error) {
	src, err := in.bytes()
	if err != nil {
		return nil, err
	}
	return token.NewFile(in.name(), src), nil
}

func (in Input) mount() preprocessor.Mount {
	if in.FS != nil {
		return preprocessor.Mount{Name: ".", FS: in.FS}
	}
	if in.name() == "<stdin>" {
		return preprocessor.Mount{}
	}
	dir := filepath.Dir(in.Name)
	if dir == "" {
		dir = "."
	}
	return preprocessor.Mount{Name: dir, FS: os.DirFS(dir)}
}
