package vcx

import (
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/vertex-language/amdgpu/feature"
	"github.com/vertex-language/ptx"

	"github.com/vertex-language/vcx/preprocessor"
	"github.com/vertex-language/vcx/types"
)

// Language is the language an input is written in. C++ is the default;
// the two offload languages are C++ with execution spaces, a launch
// syntax and a device pass, told apart by the file's extension the way
// nvcc and hipcc tell them, or named outright with -x.
type Language uint8

const (
	LangCXX Language = iota
	LangCUDA
	LangHIP
)

func (l Language) String() string {
	switch l {
	case LangCUDA:
		return "cuda"
	case LangHIP:
		return "hip"
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
	}
	return LangCXX, fmt.Errorf("unknown language %q (supported: c++, cuda, hip)", s)
}

// Language is the language the input's name says it is written in: .cu
// and .cuh are CUDA, .hip is HIP, and everything else is C++.
func (in Input) Language() Language {
	switch strings.ToLower(filepath.Ext(in.Name)) {
	case ".cu", ".cuh":
		return LangCUDA
	case ".hip":
		return LangHIP
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

// OffloadArch is one device to compile kernels for: an sm_NN for NVIDIA
// or a gfxNNN for AMD, as --offload-arch names them.
type OffloadArch struct {
	Name string
	SM   ptx.Target   // set when the device is NVIDIA's
	ASIC feature.ASIC // set when the device is AMD's
}

// ISA is the instruction set the device runs.
func (a OffloadArch) ISA() types.DeviceISA {
	switch {
	case a.SM.SM != 0:
		return types.NVPTX
	case a.ASIC != 0:
		return types.AMDGCN
	}
	return types.NoDevice
}

// Target is the device target the arch's kernels are compiled for.
func (a OffloadArch) Target() Target {
	if a.ASIC != 0 {
		return targets["amdgcn-hsa"]
	}
	return targets["nvptx64-cuda"]
}

// ParseOffloadArch reads an --offload-arch argument: sm_75, sm_90a,
// compute_80 (a virtual architecture, accepted as its real one), gfx942.
func ParseOffloadArch(s string) (OffloadArch, error) {
	name := strings.ToLower(strings.TrimSpace(s))
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
	return OffloadArch{}, fmt.Errorf("unknown offload arch %q (an sm_NN or a gfxNNN)", s)
}

// defaultOffloadArch is the device compiled for when none is named: the
// oldest NVIDIA SM every CUDA 12 driver still runs, as nvcc's own default
// is; a HIP unit has no default, since no AMD GPU runs another's code.
func (c *Compiler) offloadArch(lang Language) (OffloadArch, error) {
	if c.OffloadArch != "" {
		return ParseOffloadArch(c.OffloadArch)
	}
	if lang == LangHIP {
		return OffloadArch{}, fmt.Errorf("a HIP unit names its device with --offload-arch (gfx942, gfx90a, ...); there is no AMD GPU every kernel runs on")
	}
	return OffloadArch{Name: "sm_52", SM: ptx.SM52}, nil
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
//go:embed all:include/cuda all:include/hip
var offloadHeaderFS embed.FS

// offloadHeaders is the language's embedded include directory.
func offloadHeaders(lang Language) (SystemInclude, bool) {
	var dir string
	switch lang {
	case LangCUDA:
		dir = "include/cuda"
	case LangHIP:
		dir = "include/hip"
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
	}
	return ""
}
