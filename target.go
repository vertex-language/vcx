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

	// ContainerPTX is NVIDIA's textual ISA: what a CUDA device
	// compilation produces, which the driver assembles at load time.
	ContainerPTX

	// ContainerMetallib is Apple's GPU library: AIR bitcode, one module
	// per function, which Metal loads with newLibraryWithURL:.
	ContainerMetallib
)

func (c Container) String() string {
	switch c {
	case ContainerELF:
		return "elf"
	case ContainerMachO:
		return "macho"
	case ContainerPE:
		return "pe"
	case ContainerPTX:
		return "ptx"
	case ContainerMetallib:
		return "metallib"
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
	"aarch64-android": {
		Name:      "aarch64-android",
		Arch:      "arm64",
		OS:        "android",
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

	// The two device targets: what the device pass of a CUDA or HIP unit
	// is compiled for. Both are GNU-dialect, Itanium-ABI targets, as
	// clang's are, whatever the host is; the host pass of the same unit
	// keeps the host's target. Which SM or GFX processor is in
	// Compiler.OffloadArch, the way a CPU's feature set is not in its
	// target name either.
	"nvptx64-cuda": {
		Name:      "nvptx64-cuda",
		Arch:      "nvptx64",
		OS:        "cuda",
		Container: ContainerPTX,
		ABI:       ABIItanium,
	},
	"amdgcn-hsa": {
		Name:      "amdgcn-hsa",
		Arch:      "amdgcn",
		OS:        "hsa",
		Container: ContainerELF,
		ABI:       ABIItanium,
	},
	"air64-apple": {
		Name:      "air64-apple",
		Arch:      "air64",
		OS:        "apple",
		Container: ContainerMetallib,
		ABI:       ABIItanium,
	},
}

// Device reports whether t is a GPU: a target no host program runs on,
// whose code is a kernel image the host loads.
func (t Target) Device() bool {
	return t.Arch == "nvptx64" || t.Arch == "amdgcn" || t.Arch == "air64"
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
	switch name {
	case "nvptx64", "nvptx64-nvidia-cuda", "nvptx":
		name = "nvptx64-cuda"
	case "amdgcn", "amdgcn-amd-amdhsa", "amdgcn-hsa":
		name = "amdgcn-hsa"
	case "air64", "air64-apple-macosx", "air64-apple":
		name = "air64-apple"
	}

	if t, ok := targets[name]; ok {
		return t, nil
	}
	return Target{}, fmt.Errorf("unknown target %q", name)
}
