//go:build windows

package vcx_test

// The execution oracle for the cuda corpus: the CUDA driver, which JITs
// the PTX vcx produced and runs it on the GPU. nvcuda.dll ships with
// every NVIDIA driver and syscall reaches it without a toolkit or cgo; a
// machine without one, or with one and no GPU, skips.

import (
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

type cudaDriver struct {
	dll *syscall.LazyDLL

	init, deviceGet, deviceGetAttribute, ctxCreate, ctxSynchronize, ctxSetCurrent,
	moduleLoadDataEx, moduleUnload, moduleGetFunction, memAlloc, memFree,
	memcpyHtoD, memcpyDtoH, memsetD32, launchKernel, getErrorString *syscall.LazyProc

	ctx  uintptr
	arch string // sm_NN of the device
}

var (
	driverOnce bool
	driver     *cudaDriver
	driverSkip string
)

// gpu opens the driver once per test binary and skips the test when
// there is nothing to run on. A context is current on an OS thread, so
// the test's thread is pinned and the context made current on it.
func gpu(t *testing.T) *cudaDriver {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	if !driverOnce {
		driverOnce = true
		driver, driverSkip = openDriver()
	}
	if driver == nil {
		t.Skip(driverSkip)
	}
	if r, _, _ := driver.ctxSetCurrent.Call(driver.ctx); r != 0 {
		t.Fatalf("cuCtxSetCurrent: %s", driver.errString(r))
	}
	return driver
}

func openDriver() (*cudaDriver, string) {
	c := &cudaDriver{dll: syscall.NewLazyDLL("nvcuda.dll")}
	if err := c.dll.Load(); err != nil {
		return nil, "no nvcuda.dll: " + err.Error()
	}
	procs := map[string]**syscall.LazyProc{
		"cuInit": &c.init, "cuDeviceGet": &c.deviceGet, "cuDeviceGetAttribute": &c.deviceGetAttribute,
		"cuCtxCreate_v2": &c.ctxCreate, "cuCtxSynchronize": &c.ctxSynchronize, "cuCtxSetCurrent": &c.ctxSetCurrent,
		"cuModuleLoadDataEx": &c.moduleLoadDataEx, "cuModuleUnload": &c.moduleUnload, "cuModuleGetFunction": &c.moduleGetFunction,
		"cuMemAlloc_v2": &c.memAlloc, "cuMemFree_v2": &c.memFree,
		"cuMemcpyHtoD_v2": &c.memcpyHtoD, "cuMemcpyDtoH_v2": &c.memcpyDtoH, "cuMemsetD32_v2": &c.memsetD32,
		"cuLaunchKernel": &c.launchKernel, "cuGetErrorString": &c.getErrorString,
	}
	for name, p := range procs {
		*p = c.dll.NewProc(name)
		if err := (*p).Find(); err != nil {
			return nil, "nvcuda.dll lacks " + name
		}
	}
	if r, _, _ := c.init.Call(0); r != 0 {
		return nil, "cuInit: " + c.errString(r)
	}
	var dev int32
	if r, _, _ := c.deviceGet.Call(uintptr(unsafe.Pointer(&dev)), 0); r != 0 {
		return nil, "cuDeviceGet: " + c.errString(r)
	}
	var major, minor int32
	const attrMajor, attrMinor = 75, 76
	c.deviceGetAttribute.Call(uintptr(unsafe.Pointer(&major)), attrMajor, uintptr(dev))
	c.deviceGetAttribute.Call(uintptr(unsafe.Pointer(&minor)), attrMinor, uintptr(dev))
	sm := int(major*10 + minor)
	if sm > 90 {
		sm = 90
	}
	c.arch = fmt.Sprintf("sm_%d", sm)
	if r, _, _ := c.ctxCreate.Call(uintptr(unsafe.Pointer(&c.ctx)), 0, uintptr(dev)); r != 0 {
		return nil, "cuCtxCreate: " + c.errString(r)
	}
	return c, ""
}

func (c *cudaDriver) errString(r uintptr) string {
	var p *byte
	c.getErrorString.Call(r, uintptr(unsafe.Pointer(&p)))
	if p == nil {
		return fmt.Sprintf("CUDA error %d", r)
	}
	var b []byte
	for q := unsafe.Pointer(p); *(*byte)(q) != 0; q = unsafe.Add(q, 1) {
		b = append(b, *(*byte)(q))
	}
	return string(b)
}

func (c *cudaDriver) check(what string, r uintptr) error {
	if r != 0 {
		return fmt.Errorf("%s: %s", what, c.errString(r))
	}
	return nil
}

// load JITs the text, or loads a cubin as it is; the log is what ptxas
// would have said.
func (c *cudaDriver) load(src string) (uintptr, error) {
	image := append([]byte(src), 0)
	logBuf := make([]byte, 8192)
	const jitErrorLogBuffer, jitErrorLogBufferSizeBytes = 5, 6
	opts := [2]uint32{jitErrorLogBuffer, jitErrorLogBufferSizeBytes}
	vals := [2]uintptr{uintptr(unsafe.Pointer(&logBuf[0])), uintptr(len(logBuf))}
	var h uintptr
	r, _, _ := c.moduleLoadDataEx.Call(
		uintptr(unsafe.Pointer(&h)), uintptr(unsafe.Pointer(&image[0])),
		2, uintptr(unsafe.Pointer(&opts[0])), uintptr(unsafe.Pointer(&vals[0])))
	runtime.KeepAlive(image)
	runtime.KeepAlive(logBuf)
	if r != 0 {
		log := strings.TrimRight(string(logBuf[:strings.IndexByte(string(logBuf), 0)]), "\n")
		return 0, fmt.Errorf("cuModuleLoadDataEx: %s\n%s\n--- ptx ---\n%s", c.errString(r), log, src)
	}
	return h, nil
}

func (c *cudaDriver) function(mod uintptr, name string) (uintptr, error) {
	var f uintptr
	cname := append([]byte(name), 0)
	r, _, _ := c.moduleGetFunction.Call(uintptr(unsafe.Pointer(&f)), mod, uintptr(unsafe.Pointer(&cname[0])))
	runtime.KeepAlive(cname)
	return f, c.check("cuModuleGetFunction "+name, r)
}

// launch runs f over grid×block work-items with the given arguments,
// each a pointer to a host value of the parameter's type, and waits.
func (c *cudaDriver) launch(f uintptr, grid, block [3]uint32, shmem uint32, args ...unsafe.Pointer) error {
	params := make([]uintptr, len(args))
	for i, a := range args {
		params[i] = uintptr(a)
	}
	var pp uintptr
	if len(params) > 0 {
		pp = uintptr(unsafe.Pointer(&params[0]))
	}
	r, _, _ := c.launchKernel.Call(f,
		uintptr(grid[0]), uintptr(grid[1]), uintptr(grid[2]),
		uintptr(block[0]), uintptr(block[1]), uintptr(block[2]),
		uintptr(shmem), 0, pp, 0)
	runtime.KeepAlive(params)
	runtime.KeepAlive(args)
	if err := c.check("cuLaunchKernel", r); err != nil {
		return err
	}
	r, _, _ = c.ctxSynchronize.Call()
	return c.check("cuCtxSynchronize", r)
}

// driverPresent reports whether the machine has a driver and a GPU to
// run on, and otherwise why not.
func driverPresent() (*cudaDriver, string) {
	if !driverOnce {
		driverOnce = true
		driver, driverSkip = openDriver()
	}
	return driver, driverSkip
}

// TestCUDACorpusRuns runs every kernel of the corpus on the GPU: the
// output buffer starts zeroed, the kernel writes it, and what comes back
// is what the file expects.
func TestCUDACorpusRuns(t *testing.T) {
	cases := cudaCases(t)
	gpu(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.expect) == 0 {
				t.Skip("no expectation")
			}
			// A subtest is its own goroutine: the context is made current
			// on its thread.
			c := gpu(t)
			runKernelCase(t, c, tc, ptxOf(t, tc, c.arch))
		})
	}
}

// runKernelCase loads the PTX, runs the case's kernel over a zeroed
// buffer with the launch its header names, and compares what came back.
func runKernelCase(t *testing.T, c *cudaDriver, tc cudaCase, src string) {
	t.Helper()
	mod, err := c.load(src)
	if err != nil {
		t.Fatal(err)
	}
	defer c.moduleUnload.Call(mod)
	f, err := c.function(mod, "_Z4testPi")
	if err != nil {
		t.Fatal(err)
	}
	size := uintptr(len(tc.expect)) * 4
	var dptr uint64
	if r, _, _ := c.memAlloc.Call(uintptr(unsafe.Pointer(&dptr)), size); r != 0 {
		t.Fatal(c.check("cuMemAlloc", r))
	}
	defer c.memFree.Call(uintptr(dptr))
	if r, _, _ := c.memsetD32.Call(uintptr(dptr), 0, uintptr(len(tc.expect))); r != 0 {
		t.Fatal(c.check("cuMemsetD32", r))
	}
	if err := c.launch(f, tc.grid, tc.block, tc.shmem, unsafe.Pointer(&dptr)); err != nil {
		t.Fatalf("%v\n--- ptx ---\n%s", err, src)
	}
	got := make([]int32, len(tc.expect))
	if r, _, _ := c.memcpyDtoH.Call(uintptr(unsafe.Pointer(&got[0])), uintptr(dptr), size); r != 0 {
		t.Fatal(c.check("cuMemcpyDtoH", r))
	}
	for i := range got {
		if got[i] != tc.expect[i] {
			t.Fatalf("out[%d] = %d, want %d\nout = %v\n--- ptx ---\n%s", i, got[i], tc.expect[i], got, src)
		}
	}
}
