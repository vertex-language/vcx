package vcx_test

// The CUDA and HIP ladders: tests/cuda/NNN-*.cu and tests/hip/NNN-*.hip,
// whole programs with a main that allocates, launches and prints.
//
// Every program is compiled by vcx through both passes, everywhere: the
// device image and the host object that embeds it must come out. Where the
// vendor's compiler and a GPU are present, the program is also built by
// that compiler and by vcx, both are run, and stdout and the exit status
// are compared, as the C++ ladder compares with clang++.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/vertex-language/vcx"
)

func TestCUDA(t *testing.T) {
	files := ladder(t, filepath.Join("tests", "cuda"), "*.cu")
	nvcc, why := findNvcc(t)
	for _, f := range files {
		t.Run(strings.TrimSuffix(filepath.Base(f), ".cu"), func(t *testing.T) {
			compileBothPasses(t, f, "sm_75")
			if nvcc == nil {
				t.Skip(why)
			}
			dir := t.TempDir()
			oracle := filepath.Join(dir, exeName("nvcc"))
			if out, err := nvcc(f, oracle); err != nil {
				t.Fatalf("nvcc refused it: %v\n%s", err, out)
			}
			want := runProgram(t, oracle)
			got := runProgram(t, buildOffload(t, f, dir, ""))
			compareRuns(t, got, want, "nvcc's")
		})
	}
}

func TestHIP(t *testing.T) {
	files := ladder(t, filepath.Join("tests", "hip"), "*.hip")
	hipcc, arch, why := findHipcc()
	for _, f := range files {
		t.Run(strings.TrimSuffix(filepath.Base(f), ".hip"), func(t *testing.T) {
			compileBothPasses(t, f, "gfx942")
			if hipcc == "" {
				t.Skip(why)
			}
			dir := t.TempDir()
			oracle := filepath.Join(dir, exeName("hipcc"))
			if out, err := exec.Command(hipcc, "--offload-arch="+arch, "-o", oracle, f).CombinedOutput(); err != nil {
				t.Fatalf("hipcc refused it: %v\n%s", err, out)
			}
			want := runProgram(t, oracle)
			got := runProgram(t, buildOffload(t, f, dir, arch))
			compareRuns(t, got, want, "hipcc's")
		})
	}
}

// compileBothPasses is the check every machine makes: the device pass
// for arch and the host pass that embeds its image, without diagnostics.
func compileBothPasses(t *testing.T, path, arch string) {
	t.Helper()
	vcxBuild(t, "build", "-c", "--offload-arch", arch, "-o", filepath.Join(t.TempDir(), "unit.o"), path)
}

// buildOffload builds the program with vcx, for arch or the default.
func buildOffload(t *testing.T, path, dir, arch string) string {
	t.Helper()
	bin := filepath.Join(dir, exeName("vcx"))
	args := []string{"build", "-o", bin}
	if arch != "" {
		args = append(args, "--offload-arch", arch)
	}
	vcxBuild(t, append(args, path)...)
	return bin
}

func exeName(stem string) string {
	if runtime.GOOS == "windows" {
		return stem + ".exe"
	}
	return stem
}

// findNvcc is nvcc as a function that builds a program, where a toolkit and
// an NVIDIA GPU are both here, or the reason the comparison is skipped.
// On Windows nvcc's host side is cl, which it finds through -ccbin and the
// environment findMSVC gives it.
func findNvcc(t *testing.T) (func(src, bin string) ([]byte, error), string) {
	tk, ok := vcx.FindCUDAToolkit()
	if !ok {
		return nil, "no CUDA toolkit: the programs are compiled, not run"
	}
	nvcc := filepath.Join(tk.Dir, "bin", exeName("nvcc"))
	if _, err := os.Stat(nvcc); err != nil {
		return nil, "no nvcc in the CUDA toolkit: the programs are compiled, not run"
	}
	if out, err := exec.Command("nvidia-smi", "-L").Output(); err != nil || !strings.Contains(string(out), "GPU") {
		return nil, "no NVIDIA GPU: the programs are compiled, not run"
	}
	var env []string
	var ccbin []string
	if runtime.GOOS == "windows" {
		cl, ok := findMSVC(t)
		if !ok {
			return nil, "no cl.exe for nvcc's host side: the programs are compiled, not run"
		}
		ccbin = []string{"-ccbin", filepath.Dir(cl.exe)}
		env = append(os.Environ(), "INCLUDE="+cl.include, "LIB="+cl.lib)
	}
	return func(src, bin string) ([]byte, error) {
		args := append(append([]string{}, ccbin...), "-arch=sm_75", "-o", bin, src)
		cmd := exec.Command(nvcc, args...)
		cmd.Env = env
		return cmd.CombinedOutput()
	}, ""
}

// findHipcc is hipcc and the architecture of the machine's AMD GPU, which
// rocm_agent_enumerator names, or the reason the comparison is skipped.
func findHipcc() (hipcc, arch, why string) {
	hipcc, err := exec.LookPath("hipcc")
	if err != nil {
		return "", "", "no hipcc: the programs are compiled, not run"
	}
	out, err := exec.Command("rocm_agent_enumerator").Output()
	if err != nil {
		return "", "", "no rocm_agent_enumerator to name the GPU: the programs are compiled, not run"
	}
	for _, line := range strings.Fields(string(out)) {
		if strings.HasPrefix(line, "gfx") && line != "gfx000" {
			return hipcc, line, ""
		}
	}
	return "", "", "no AMD GPU: the programs are compiled, not run"
}
