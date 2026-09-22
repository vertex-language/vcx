package vcx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vertex-language/elf"
	elflink "github.com/vertex-language/elf/link"
	"github.com/vertex-language/macho"
	macholink "github.com/vertex-language/macho/link"
	"github.com/vertex-language/pe"
	pelink "github.com/vertex-language/pe/link"

	"github.com/vertex-language/vcc/sysroot"
)

// The link, below VIR: the three container formats are three
// vertex-language linkers, each taking bytes and returning bytes, so an
// object this process just produced never touches the filesystem on
// its way in. There is no cc on the path and nothing to detect; the
// platform's own libraries are found the way vcc finds them, through
// vcc/sysroot.

// LinkParams is one link.
type LinkParams struct {
	// Objects are the images to link, in order: paths, or bytes an
	// earlier step produced.
	Objects []Input

	// Output is the executable's path.
	Output string

	// Entry names the entry point; empty is the platform's own.
	Entry string

	// Static asks for a static link where the format has the choice.
	Static bool

	// LibDirs and Libs are -L and -l, resolved against LibDirs and then
	// the platform's directories, and added after every object.
	LibDirs []string
	Libs    []string
}

// Link produces an executable from objects for the compiler's target.
func (c *Compiler) Link(p LinkParams) error {
	if len(p.Objects) == 0 {
		return fmt.Errorf("nothing to link")
	}
	if p.Output == "" {
		return fmt.Errorf("link needs an output path")
	}
	t, err := c.target()
	if err != nil {
		return err
	}
	if t.Device() {
		return fmt.Errorf("a device image is not linked: it is loaded by the host program that embeds it")
	}
	switch t.Container {
	case ContainerPE:
		return c.linkPE(t, p)
	case ContainerELF:
		return c.linkELF(t, p)
	case ContainerMachO:
		return c.linkMachO(t, p)
	}
	return fmt.Errorf("no linker for %s", t.Container)
}

// libraryDirs is where a -l name is looked for: the caller's first,
// then the platform's.
func (c *Compiler) libraryDirs(t Target, p LinkParams) []string {
	dirs := append([]string(nil), p.LibDirs...)
	return append(dirs, sysroot.LibraryDirs(nil, t.Name, !c.Freestanding)...)
}

// libraryNames is the -l list one link resolves: the caller's, then the
// platform's default runtime for the ones it did not already name.
func (c *Compiler) libraryNames(t Target, p LinkParams) []string {
	names := append([]string(nil), p.Libs...)
	named := map[string]bool{}
	for _, n := range names {
		named[n] = true
	}
	for _, n := range sysroot.DefaultLibraries(nil, t.Name, !c.Freestanding) {
		if !named[n] {
			names = append(names, n)
		}
	}
	return names
}

// libraryFiles is the filenames "-l name" can mean on the target, in
// the order they are tried: MSVC's convention has foo.lib the import
// library and libfoo.lib the static one.
func libraryFiles(t Target, name string, static bool) []string {
	archive := "lib" + name + ".a"
	switch t.Container {
	case ContainerPE:
		if static {
			return []string{"lib" + name + ".lib", archive, name + ".lib"}
		}
		return []string{name + ".lib", "lib" + name + ".lib", archive}
	case ContainerMachO:
		if static {
			return []string{archive}
		}
		return []string{"lib" + name + ".tbd", "lib" + name + ".dylib", archive}
	case ContainerELF:
		if static {
			return []string{archive}
		}
		return []string{"lib" + name + ".so", archive}
	}
	return []string{archive}
}

// libraries resolves every -l name of a link and reads it.
func (c *Compiler) libraries(t Target, p LinkParams) ([]Input, error) {
	dirs := c.libraryDirs(t, p)
	var out []Input
	for _, name := range c.libraryNames(t, p) {
		files := libraryFiles(t, name, p.Static)
		found := false
		for _, dir := range dirs {
			for _, base := range files {
				path := filepath.Join(dir, base)
				data, err := os.ReadFile(path)
				if err == nil {
					out = append(out, Input{Name: path, Data: data})
					found = true
					break
				}
				if !os.IsNotExist(err) {
					return nil, fmt.Errorf("%s: %w", path, err)
				}
			}
			if found {
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("cannot find -l%s: no %s in %s", name, strings.Join(files, ", "), strings.Join(dirs, "; "))
		}
	}
	return out, nil
}

// addInputs hands each input to a linker's add function, in order.
func addInputs(add func(string, []byte) error, objs []Input) error {
	for _, o := range objs {
		data, err := o.bytes()
		if err != nil {
			return err
		}
		if err := add(o.Name, data); err != nil {
			return err
		}
	}
	return nil
}

func linkErr(err error) error {
	if strings.HasPrefix(err.Error(), "link:") {
		return err
	}
	return fmt.Errorf("link: %w", err)
}

func (c *Compiler) linkPE(t Target, p LinkParams) error {
	var m pe.Machine
	switch t.Arch {
	case "amd64":
		m = pe.MachineAMD64
	case "arm64":
		m = pe.MachineARM64
	case "i386":
		m = pe.MachineI386
	default:
		return fmt.Errorf("pe: no linker for %s", t.Arch)
	}
	l, err := pelink.New(pe.Target{Machine: m, SubArch: m.SubArch(), ABI: pe.ABIMSVC, OS: pe.OSWindows})
	if err != nil {
		return linkErr(err)
	}
	if p.Entry != "" {
		l.SetEntry(p.Entry)
	}
	// The PE linker resolves names of its own -- a /DEFAULTLIB inside a
	// CRT object -- and needs the search path for that.
	l.SetLibPath(c.libraryDirs(t, p)...)
	libs, err := c.libraries(t, p)
	if err != nil {
		return err
	}
	if err := addInputs(l.AddObject, p.Objects); err != nil {
		return linkErr(err)
	}
	if err := addInputs(l.AddArchive, libs); err != nil {
		return linkErr(err)
	}
	img, err := l.Link()
	if err != nil {
		return linkErr(err)
	}
	b, err := img.Bytes()
	if err != nil {
		return linkErr(err)
	}
	return os.WriteFile(p.Output, b, 0o755)
}

func (c *Compiler) linkELF(t Target, p LinkParams) error {
	var arch elf.Arch
	switch t.Arch {
	case "amd64":
		arch = elf.ArchAMD64
	case "arm64":
		arch = elf.ArchARM64
	case "i386":
		arch = elf.ArchI386
	default:
		return fmt.Errorf("elf: no linker for %s", t.Arch)
	}
	target := elf.Target{Arch: arch, Class: elf.ELFCLASS64, Endian: elf.EndianLittle}
	if arch == elf.ArchI386 {
		target.Class = elf.ELFCLASS32
	}
	l := elflink.New(target)
	if p.Entry != "" {
		l.SetEntry(p.Entry)
	}
	l.Options().Static = p.Static
	libs, err := c.libraries(t, p)
	if err != nil {
		return err
	}
	if err := addInputs(l.AddFile, p.Objects); err != nil {
		return linkErr(err)
	}
	if err := addInputs(l.AddFile, libs); err != nil {
		return linkErr(err)
	}
	img, err := l.Link()
	if err != nil {
		return linkErr(err)
	}
	return os.WriteFile(p.Output, img.Bytes(), 0o755)
}

func (c *Compiler) linkMachO(t Target, p LinkParams) error {
	var cpu macho.CPU
	var sub macho.SubCPU
	switch t.Arch {
	case "amd64":
		cpu, sub = macho.CPU_TYPE_X86_64, macho.CPU_SUBTYPE_X86_64_ALL
	case "arm64":
		cpu, sub = macho.CPU_TYPE_ARM64, macho.CPU_SUBTYPE_ARM64_ALL
	default:
		return fmt.Errorf("macho: no linker for %s", t.Arch)
	}
	target := macho.Target{CPU: cpu, SubCPU: sub, Platform: macho.PlatformMacOS, Endian: macho.LittleEndian}
	if v, err := macho.ParseVersion(macOSMinimum(t)); err == nil {
		target.MinOS = v
	}
	l, err := macholink.New(target)
	if err != nil {
		return linkErr(err)
	}
	if p.Entry != "" {
		l.SetEntry("_" + p.Entry)
	}
	if !c.Freestanding {
		if sdk, ok := sysroot.SDK(nil); ok {
			l.SetSDK(sdk)
			if data, err := os.ReadFile(filepath.Join(sdk, "usr/lib/libSystem.tbd")); err == nil {
				l.AddStub("libSystem", data)
			}
			// The C++ runtime, as clang++ links it for every C++ program:
			// operator new and delete, the exception and RTTI support of
			// libc++abi, which libc++ re-exports, and the library itself.
			if data, err := os.ReadFile(filepath.Join(sdk, "usr/lib/libc++.tbd")); err == nil {
				l.AddStub("libc++", data)
			}
		}
	}
	libs, err := c.libraries(t, p)
	if err != nil {
		return err
	}
	if err := addInputs(l.AddFile, p.Objects); err != nil {
		return linkErr(err)
	}
	if err := addInputs(l.AddFile, libs); err != nil {
		return linkErr(err)
	}
	img, err := l.Link()
	if err != nil {
		return linkErr(err)
	}
	b, err := img.Bytes()
	if err != nil {
		return linkErr(err)
	}
	return os.WriteFile(p.Output, b, 0o755)
}
