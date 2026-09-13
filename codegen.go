package vcx

import (
	"bytes"
	"fmt"

	amd64elf "github.com/vertex-language/amd64/obj/elf"
	amd64macho "github.com/vertex-language/amd64/obj/macho"
	amd64pe "github.com/vertex-language/amd64/obj/pe"
	arm64elf "github.com/vertex-language/arm64/obj/elf"
	arm64macho "github.com/vertex-language/arm64/obj/macho"
	i386elf "github.com/vertex-language/i386/obj/elf"
	machocore "github.com/vertex-language/macho"

	"github.com/vertex-language/ir"
	amd64lower "github.com/vertex-language/ir/lower/amd64"
	arm64lower "github.com/vertex-language/ir/lower/arm64"
	i386lower "github.com/vertex-language/ir/lower/i386"
	"github.com/vertex-language/vcx/mangle"
)

// irTarget returns the VIR target layout for t.
func irTarget(t Target) (ir.Target, error) {
	switch {
	case t.Arch == "amd64" && t.OS == "windows":
		return ir.X86_64Windows, nil
	case t.Arch == "amd64" && t.OS == "macos":
		return ir.X86_64MacOS, nil
	case t.Arch == "amd64":
		return ir.X86_64Linux, nil
	case t.Arch == "arm64" && t.OS == "macos":
		return ir.AArch64MacOS, nil
	case t.Arch == "arm64":
		return ir.AArch64Linux, nil
	case t.Arch == "i386":
		return ir.I386Linux, nil
	}
	return ir.Target{}, fmt.Errorf("target %s names no VIR layout", t.Name)
}

// manglingABI returns the name mangling ABI for t.
func manglingABI(t Target) mangle.ABI {
	if t.ABI == ABIMicrosoft {
		return mangle.Microsoft
	}
	return mangle.Itanium
}

// symbolPrefix returns the symbol prefix for t ("_" on Mach-O, empty otherwise).
func symbolPrefix(t Target) string {
	if t.Container == ContainerMachO {
		return "_"
	}
	return ""
}

// macOSMinimum returns the minimum deployment target version for Mach-O binaries.
func macOSMinimum(t Target) string {
	if t.Arch == "arm64" {
		return "11.0"
	}
	return "10.13"
}

// emitObject lowers a VIR module and encodes it in the target container format.
func emitObject(m *ir.Module, t Target) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("no module to emit")
	}
	switch t.Arch {
	case "amd64":
		return amd64Object(m, t)
	case "arm64":
		return arm64Object(m, t)
	case "i386":
		return i386Object(m, t)
	}
	return nil, fmt.Errorf("target %s names no backend", t.Name)
}

func amd64Object(m *ir.Module, t Target) ([]byte, error) {
	o, err := amd64lower.Lower(m, amd64lower.Options{LibcallPrefix: symbolPrefix(t)})
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	switch t.Container {
	case ContainerELF:
		err = amd64elf.Write(&buf, o, amd64elf.Options{})
	case ContainerMachO:
		err = amd64macho.Write(&buf, o, amd64macho.Options{
			Platform: machocore.PlatformMacOS,
			MinOS:    macOSMinimum(t),
		})
	case ContainerPE:
		err = amd64pe.Write(&buf, o, amd64pe.Options{File: m.Name()})
	default:
		err = fmt.Errorf("amd64 has no writer for %s", t.Container)
	}
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func arm64Object(m *ir.Module, t Target) ([]byte, error) {
	variadic := arm64lower.VariadicAAPCS64
	if t.Container == ContainerMachO {
		variadic = arm64lower.VariadicDarwin
	}
	o, err := arm64lower.Lower(m, arm64lower.Options{
		Variadic:      variadic,
		LibcallPrefix: symbolPrefix(t),
	})
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	switch t.Container {
	case ContainerELF:
		err = arm64elf.Write(&buf, o, arm64elf.Options{})
	case ContainerMachO:
		err = arm64macho.Write(&buf, o, arm64macho.Options{
			Platform:    machocore.PlatformMacOS,
			MinOS:       macOSMinimum(t),
			Subsections: true,
		})
	default:
		err = fmt.Errorf("arm64 has no writer for %s", t.Container)
	}
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func i386Object(m *ir.Module, t Target) ([]byte, error) {
	o, err := i386lower.Lower(m, i386lower.Options{})
	if err != nil {
		return nil, err
	}
	if t.Container != ContainerELF {
		return nil, fmt.Errorf("i386 has no writer for %s", t.Container)
	}
	var buf bytes.Buffer
	if err := i386elf.Write(&buf, o, i386elf.Options{}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
