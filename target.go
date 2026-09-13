package vcx

import (
	"fmt"
	"runtime"
	"strings"
)

// ABI identifies the C++ ABI used for layout, mangling, vtables, and exceptions.
type ABI uint8

const (
	ABIItanium ABI = iota
	ABIMicrosoft
)

func (a ABI) String() string {
	switch a {
	case ABIItanium:
		return "itanium"
	case ABIMicrosoft:
		return "microsoft"
	}
	return "unknown"
}

// Container format
type Container uint8

const (
	ContainerELF Container = iota
	ContainerMachO
	ContainerPE
)

func (c Container) String() string {
	switch c {
	case ContainerELF:
		return "elf"
	case ContainerMachO:
		return "macho"
	case ContainerPE:
		return "pe"
	}
	return "unknown"
}

// Dialect is the family of toolchains a target's code is written for: which
// compiler's extensions its headers use, which macros say who is compiling,
// and which of the two families' tests apply to it.
//
// It is a separate axis from the ABI, the way clang keeps its environment
// apart from its C++ ABI: the two agree on every target here, and would not
// for a MinGW one, whose code is GNU and whose data model is Windows'.
type Dialect uint8

const (
	DialectGNU  Dialect = iota // gcc and clang: libstdc++, libc++, __attribute__
	DialectMSVC                // cl: the Microsoft STL, __declspec
)

func (d Dialect) String() string {
	if d == DialectMSVC {
		return "msvc"
	}
	return "gnu"
}

// Target describes the compilation target.
type Target struct {
	Name         string
	Arch         string
	OS           string
	Container    Container
	ABI          ABI
	Dialect      Dialect
	Freestanding bool
}

var targets = map[string]Target{
	"x86_64-linux": {
		Name:      "x86_64-linux",
		Arch:      "amd64",
		OS:        "linux",
		Container: ContainerELF,
		ABI:       ABIItanium,
	},
	"aarch64-linux": {
		Name:      "aarch64-linux",
		Arch:      "arm64",
		OS:        "linux",
		Container: ContainerELF,
		ABI:       ABIItanium,
	},
	"x86_64-windows": {
		Name:      "x86_64-windows",
		Arch:      "amd64",
		OS:        "windows",
		Container: ContainerPE,
		ABI:       ABIMicrosoft,
		Dialect:   DialectMSVC,
	},
	"x86_64-macos": {
		Name:      "x86_64-macos",
		Arch:      "amd64",
		OS:        "macos",
		Container: ContainerMachO,
		ABI:       ABIItanium,
	},
	"aarch64-macos": {
		Name:      "aarch64-macos",
		Arch:      "arm64",
		OS:        "macos",
		Container: ContainerMachO,
		ABI:       ABIItanium,
	},
	"x86_64-elf": {
		Name:         "x86_64-elf",
		Arch:         "amd64",
		OS:           "none",
		Container:    ContainerELF,
		ABI:          ABIItanium,
		Freestanding: true,
	},
	"i386-elf": {
		Name:         "i386-elf",
		Arch:         "i386",
		OS:           "none",
		Container:    ContainerELF,
		ABI:          ABIItanium,
		Freestanding: true,
	},
}

// DefaultTarget returns the host machine's native target.
func DefaultTarget() Target {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	case "386":
		arch = "i386"
	}

	os := runtime.GOOS
	switch os {
	case "darwin":
		os = "macos"
	}

	name := arch + "-" + os
	if t, ok := targets[name]; ok {
		return t
	}
	return targets["x86_64-linux"]
}

// TargetByName looks up a target by canonical name or alias.
func TargetByName(name string) (Target, error) {
	name = strings.ToLower(name)
	// Normalization
	name = strings.ReplaceAll(name, "amd64", "x86_64")
	name = strings.ReplaceAll(name, "darwin", "macos")

	if t, ok := targets[name]; ok {
		return t, nil
	}
	return Target{}, fmt.Errorf("unknown target %q", name)
}
