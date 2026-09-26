package vcx

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vertex-language/air"
	"github.com/vertex-language/amdgpu/feature"
	"github.com/vertex-language/ptx"

	"github.com/vertex-language/vcx/preprocessor"
	"github.com/vertex-language/vcx/types"
)

// Language is the language an input is written in. C++ is the default;
// the offload languages are C++ with execution spaces, a launch syntax and
// a device pass, told apart by the file's extension the way nvcc and hipcc
// tell them, or named outright with -x. Metal is the third, and the odd
// one: all of a .metal file is device code, so it has the device pass
// alone, and its output is a .metallib rather than an object.
type Language uint8

const (
	LangCXX Language = iota
	LangCUDA
	LangHIP
	LangMetal
	// LangObjCXX is Objective-C++: C++ with Objective-C's classes, messages,
	// blocks and ARC on top. It is a C++ unit like any other in every other
	// way, so it has the host pass alone.
	LangObjCXX
)

func (l Language) String() string {
	switch l {
	case LangCUDA:
		return "cuda"
	case LangHIP:
		return "hip"
	case LangMetal:
		return "metal"
	case LangObjCXX:
		return "objective-c++"
	}
	return "c++"
}

// offload is the language as the analysis knows it.
func (l Language) offload() types.Offload {
	switch l {
	case LangCUDA:
		return types.CUDA
	case LangHIP:
		return types.HIP
	case LangMetal:
		return types.Metal
	}
	return types.NoOffload
}

// ParseLanguage reads a -x argument.
func ParseLanguage(s string) (Language, error) {
	switch strings.ToLower(s) {
	case "", "c++", "cxx", "cpp":
		return LangCXX, nil
	case "cuda", "cu":
		return LangCUDA, nil
	case "hip":
		return LangHIP, nil
	case "metal":
		return LangMetal, nil
	case "objective-c++", "objc++", "objcxx", "mm":
		return LangObjCXX, nil
	}
	return LangCXX, fmt.Errorf("unknown language %q (supported: c++, objective-c++, cuda, hip, metal)", s)
}

// Language is the language the input's name says it is written in: .cu
// and .cuh are CUDA, .hip is HIP, .metal is Metal, and everything else is
// C++.
func (in Input) Language() Language {
	switch strings.ToLower(filepath.Ext(in.Name)) {
	case ".cu", ".cuh":
		return LangCUDA
	case ".hip":
		return LangHIP
	case ".metal":
		return LangMetal
	case ".mm":
		return LangObjCXX
	}
	return LangCXX
}

// language is the input's language, as -x overrides it.
func (c *Compiler) language(in Input) Language {
	if c.Language != LangCXX {
		return c.Language
	}
	return in.Language()
}

// OffloadArch is one device to compile kernels for: an sm_NN for NVIDIA,
// a gfxNNN for AMD, or an appleN GPU family, as --offload-arch names them.
type OffloadArch struct {
	Name   string
	SM     ptx.Target   // set when the device is NVIDIA's
	ASIC   feature.ASIC // set when the device is AMD's
	Family air.Family   // set when the device is Apple's

	// Metal is what an Apple device's library is built as: the MSL
	// version, the deployment target, fast math. The Compiler fills it.
	Metal MetalOptions
}

// MetalOptions are what a .metallib says it is, beyond the GPU family:
// the language version (-std=metal3.1), the oldest OS it loads on
// (-mmacosx-version-min=14.0), and whether float math is fast
// (-fno-fast-math turns it off, as for xcrun).
type MetalOptions struct {
	Language air.Language
	Target   air.Target
	FastMath bool
}

// ISA is the instruction set the device runs.
func (a OffloadArch) ISA() types.DeviceISA {
	switch {
	case a.SM.SM != 0:
		return types.NVPTX
	case a.ASIC != 0:
		return types.AMDGCN
	case a.Family != 0:
		return types.AIR
	}
	return types.NoDevice
}

// Target is the device target the arch's kernels are compiled for.
func (a OffloadArch) Target() Target {
	switch {
	case a.ASIC != 0:
		return targets["amdgcn-hsa"]
	case a.Family != 0:
		return targets["air64-apple"]
	}
	return targets["nvptx64-cuda"]
}

// ParseOffloadArch reads an --offload-arch argument: sm_75, sm_90a,
// compute_80 (a virtual architecture, accepted as its real one), gfx942.
func ParseOffloadArch(s string) (OffloadArch, error) {
	name := strings.ToLower(strings.TrimSpace(s))
	if rest, ok := strings.CutPrefix(name, "apple"); ok {
		n, err := strconv.Atoi(rest)
		if f := air.Family(n); err == nil && f.Valid() {
			return OffloadArch{Name: f.String(), Family: f}, nil
		}
		return OffloadArch{}, fmt.Errorf("offload arch %q: not an Apple GPU family this compiler builds for (apple7, apple8, apple9)", s)
	}
	if asic, ok := feature.ParseASIC(name); ok {
		return OffloadArch{Name: name, ASIC: asic}, nil
	}
	for _, prefix := range []string{"sm_", "compute_"} {
		rest, ok := strings.CutPrefix(name, prefix)
		if !ok {
			continue
		}
		t := ptx.Target{}
		switch {
		case strings.HasSuffix(rest, "a"):
			t.Suffix = ptx.ArchSpc
			rest = strings.TrimSuffix(rest, "a")
		case strings.HasSuffix(rest, "f"):
			t.Suffix = ptx.Family
			rest = strings.TrimSuffix(rest, "f")
		}
		n, err := strconv.Atoi(rest)
		if err != nil || n < 50 {
			return OffloadArch{}, fmt.Errorf("offload arch %q: not an SM this compiler lowers for (sm_50 and later)", s)
		}
		t.SM = n
		return OffloadArch{Name: t.String(), SM: t}, nil
	}
	return OffloadArch{}, fmt.Errorf("unknown offload arch %q (an sm_NN, a gfxNNN or an appleN)", s)
}

// offloadArch is the first device compiled for: what the single-image
// paths -- --emit ptx, the device-only pass -- use.
func (c *Compiler) offloadArch(lang Language) (OffloadArch, error) {
	archs, err := c.offloadArchs(lang)
	if err != nil {
		return OffloadArch{}, err
	}
	return archs[0], nil
}

// offloadArchs is every device compiled for, in order, without repeats.
// None named is the oldest NVIDIA SM every CUDA 12 driver still runs,
// as nvcc's own default is; a HIP unit has no default, since no AMD GPU
// runs another's code; a Metal unit's is apple7, the M1, the oldest
// family AIR's libraries are written for. One fat binary holds one
// vendor's images, and a .metallib one family's.
func (c *Compiler) offloadArchs(lang Language) ([]OffloadArch, error) {
	archs, err := c.namedArchs(lang)
	if err != nil {
		return nil, err
	}
	if lang != LangMetal {
		for _, a := range archs {
			if a.Family != 0 {
				return nil, fmt.Errorf("--offload-arch %s is an Apple GPU, and a %s unit is compiled for NVIDIA or AMD; Apple GPUs run .metal files", a.Name, lang)
			}
		}
		return archs, nil
	}
	if len(archs) > 1 {
		return nil, fmt.Errorf("a .metallib is built for one GPU family; --offload-arch names %d", len(archs))
	}
	if archs[0].Family == 0 {
		return nil, fmt.Errorf("--offload-arch %s is not an Apple GPU; a .metal file runs on apple7, apple8 or apple9", archs[0].Name)
	}
	mo, err := c.metalOptions()
	if err != nil {
		return nil, err
	}
	archs[0].Metal = mo
	return archs, nil
}

// namedArchs is the devices --offload-arch names, or the language's default.
func (c *Compiler) namedArchs(lang Language) ([]OffloadArch, error) {
	var names []string
	for _, s := range append(strings.Split(c.OffloadArch, ","), c.OffloadArchs...) {
		if s = strings.TrimSpace(s); s != "" {
			names = append(names, s)
		}
	}
	if len(names) == 0 {
		if lang == LangHIP {
			return nil, fmt.Errorf("a HIP unit names its device with --offload-arch (gfx942, gfx90a, ...); there is no AMD GPU every kernel runs on")
		}
		if lang == LangMetal {
			return []OffloadArch{{Name: "apple7", Family: air.Apple7}}, nil
		}
		return []OffloadArch{{Name: "sm_52", SM: ptx.SM52}}, nil
	}
	var out []OffloadArch
	seen := map[string]bool{}
	for _, name := range names {
		arch, err := ParseOffloadArch(name)
		if err != nil {
			return nil, err
		}
		if seen[arch.Name] {
			continue
		}
		seen[arch.Name] = true
		if len(out) > 0 && out[0].ISA() != arch.ISA() {
			return nil, fmt.Errorf("--offload-arch %s and %s are two vendors' devices; one program is built for one", out[0].Name, arch.Name)
		}
		out = append(out, arch)
	}
	return out, nil
}

// offloadPredefines are the macros that say which pass of an offload
// unit is being compiled, as clang's CUDA and HIP frontends define them.
// The host pass of a .cu is a CUDA compilation too -- __CUDACC__ says
// so in both -- and only the device pass has an architecture.
func offloadPredefines(lang Language, arch OffloadArch, device bool) []preprocessor.Predefine {
	var out []preprocessor.Predefine
	def := func(name, value string) {
		out = append(out, preprocessor.Predefine{Kind: preprocessor.PredefineDefine, Text: name + "=" + value})
	}
	switch lang {
	case LangCUDA:
		def("__CUDACC__", "1")
		def("__CUDA__", "1")
		def("__CUDACC_VER_MAJOR__", "12")
		def("__CUDACC_VER_MINOR__", "6")
		def("__CUDACC_VER_BUILD__", "0")
		def("__NVCC__", "1") // the SDK's headers ask for nvcc by this name in places clang's wrapper cannot reach
		if device {
			def("__CUDA_ARCH__", strconv.Itoa(arch.SM.SM*10))
			if arch.SM.Suffix == ptx.ArchSpc {
				def("__CUDA_ARCH_FEAT_SM"+strconv.Itoa(arch.SM.SM)+"_ALL", "1")
			}
		}
	case LangHIP:
		def("__HIP__", "1")
		def("__HIPCC__", "1")
		def("__HIP_PLATFORM_AMD__", "1")
		def("__HIP_PLATFORM_HCC__", "1")
		def("__HIP_MEMORY_SCOPE_SINGLETHREAD", "1")
		def("__HIP_MEMORY_SCOPE_WAVEFRONT", "2")
		def("__HIP_MEMORY_SCOPE_WORKGROUP", "3")
		def("__HIP_MEMORY_SCOPE_AGENT", "4")
		def("__HIP_MEMORY_SCOPE_SYSTEM", "5")
		if device {
			def("__HIP_DEVICE_COMPILE__", "1")
		}
	case LangMetal:
		// What xcrun metal defines that a program tests: the language
		// version, the AIR version the deployment target implies, the
		// deployment target, and whether math is fast.
		mo := arch.Metal
		def("__METAL__", "1")
		def("__METAL_VERSION__", strconv.Itoa(int(mo.Language)))
		def("__AIR64__", "1")
		v := mo.Target.AIR()
		def("__AIR_VERSION__", strconv.Itoa(v.Major*10000+v.Minor*100))
		os := strconv.Itoa(mo.Target.Version.Major*10000 + mo.Target.Version.Minor*100)
		def("__ENVIRONMENT_OS_VERSION_MIN_REQUIRED__", os)
		def("__ENVIRONMENT_MAC_OS_X_VERSION_MIN_REQUIRED__", os)
		def("__APPLE__", "1")
		def("__LITTLE_ENDIAN__", "1")
		fast := "0"
		if mo.FastMath {
			fast = "1"
			def("__FAST_MATH__", "1")
		}
		def("__METAL_FAST_MATH__", fast)
		def("__HAVE_BFLOAT__", "0")
		def("__MAX_BUFFERS__", "31u")
		def("__MAX_THREADGROUP_BUFFERS__", "31u")
	}
	if device {
		switch arch.ISA() {
		case types.NVPTX:
			def("__NVPTX__", "1")
			def("__PTX__", "1")
		case types.AMDGCN:
			def("__AMDGCN__", "1")
			def("__AMDGPU__", "1")
			def("__"+arch.Name+"__", "1")
			def("__amdgcn_processor__", `"`+arch.Name+`"`)
			def("__amdgcn_target_id__", `"amdgcn-amd-amdhsa--`+arch.Name+`"`)
			wave := strconv.Itoa(int(arch.ASIC.DefaultWaveSize()))
			def("__AMDGCN_WAVEFRONT_SIZE__", wave)
			def("__AMDGCN_WAVEFRONT_SIZE", wave)
		}
	}
	return out
}

// The offload languages' own headers: the wrapper every unit of the
// language reads before its first line, the way clang force-includes its
// __clang_cuda_runtime_wrapper.h, and the minimal cuda_runtime.h and
// hip_runtime.h a unit reads when no SDK is installed.
//
//go:embed all:include/cuda all:include/hip all:include/metal
var offloadHeaderFS embed.FS

// offloadHeaders is the language's embedded include directory.
func offloadHeaders(lang Language) (SystemInclude, bool) {
	var dir string
	switch lang {
	case LangCUDA:
		dir = "include/cuda"
	case LangHIP:
		dir = "include/hip"
	case LangMetal:
		dir = "include/metal"
	default:
		return SystemInclude{}, false
	}
	sub, err := fs.Sub(offloadHeaderFS, dir)
	if err != nil {
		panic("vcx: embedded " + dir + " headers missing: " + err.Error())
	}
	return SystemInclude{Name: "<vcx-" + lang.String() + ">", FS: sub}, true
}

// offloadWrapper is the header read before the unit's first line.
func offloadWrapper(lang Language) string {
	switch lang {
	case LangCUDA:
		return "__vcx_cuda_runtime_wrapper.h"
	case LangHIP:
		return "__vcx_hip_runtime_wrapper.h"
	case LangMetal:
		return "__vcx_metal_wrapper.h"
	}
	return ""
}

// metalOptions is the Compiler's Metal flags as a library's options: MSL
// 3.0 and macOS 13 unless -std and -mmacosx-version-min say otherwise, and
// fast math unless -fno-fast-math, as xcrun defaults.
func (c *Compiler) metalOptions() (MetalOptions, error) {
	mo := MetalOptions{Language: air.MSL30, Target: air.Target{OS: air.MacOS, Version: air.OS(13, 0)}, FastMath: !c.NoFastMath}
	if c.MetalStd != "" {
		l, err := parseMetalStd(c.MetalStd)
		if err != nil {
			return mo, err
		}
		mo.Language = l
	}
	if c.MinOS != "" {
		var major, minor int
		if n, _ := fmt.Sscanf(c.MinOS, "%d.%d", &major, &minor); n == 0 {
			return mo, fmt.Errorf("-mmacosx-version-min=%s: not a version", c.MinOS)
		}
		mo.Target.Version = air.OS(major, minor)
		if !mo.Target.Valid() {
			return mo, fmt.Errorf("-mmacosx-version-min=%s: Metal libraries are written for macOS 13.0 and later", c.MinOS)
		}
	}
	return mo, nil
}

// parseMetalStd reads -std=metal3.1 and the other versions xcrun takes.
func parseMetalStd(s string) (air.Language, error) {
	for _, l := range air.Languages {
		if strings.EqualFold(s, l.Std()) {
			return l, nil
		}
	}
	var names []string
	for _, l := range air.Languages {
		names = append(names, l.Std())
	}
	return 0, fmt.Errorf("-std=%s: not an MSL version this compiler writes (%s)", s, strings.Join(names, ", "))
}
