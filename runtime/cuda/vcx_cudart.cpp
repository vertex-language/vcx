// vcx_cudart.cpp -- the CUDA runtime a program built by vcx links when
// no toolkit is installed: the cudart entry points a compiled unit
// calls, over the driver API in nvcuda.dll or libcuda.so, which every
// NVIDIA driver ships. It is compiled by vcx itself at link time and
// linked into the program as any object is.
//
// It answers the registration a unit's constructor makes
// (__cudaRegisterFatBinary and the rest), the launch a stub makes
// (__vcx_cudaLaunch, __vcx_cudaPushCallConfiguration,
// __cudaPopCallConfiguration), and the runtime API the host code calls.
// Modules are loaded lazily, at the first launch or symbol access, so
// that a program that never touches the GPU never opens the driver.
//
// What it does not do: multiple devices past cudaSetDevice, textures,
// graphs, and the rest of cudart's surface a small program never calls.

#include <cuda_runtime.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* ---- the driver ------------------------------------------------------- */

typedef int CUresult;
typedef int CUdevice;
typedef struct CUctx_st *CUcontext;
typedef struct CUmod_st *CUmodule;
typedef struct CUfunc_st *CUfunction;
typedef struct CUstream_st *CUstream;
typedef struct CUevent_st *CUevent;
typedef unsigned long long CUdeviceptr;

extern "C" {
#if defined(_WIN32)
void *__stdcall LoadLibraryA(const char *name);
void *__stdcall GetProcAddress(void *module, const char *name);
#else
void *dlopen(const char *name, int flags);
void *dlsym(void *handle, const char *name);
#endif
}

static struct Driver {
  int loaded;
  CUresult (*Init)(unsigned flags);
  CUresult (*DeviceGetCount)(int *count);
  CUresult (*DeviceGet)(CUdevice *dev, int ordinal);
  CUresult (*DeviceGetName)(char *name, int len, CUdevice dev);
  CUresult (*DeviceGetAttribute)(int *v, int attr, CUdevice dev);
  CUresult (*DeviceTotalMem)(size_t *bytes, CUdevice dev);
  CUresult (*DevicePrimaryCtxRetain)(CUcontext *ctx, CUdevice dev);
  CUresult (*CtxSetCurrent)(CUcontext ctx);
  CUresult (*CtxSynchronize)(void);
  CUresult (*ModuleLoadDataEx)(CUmodule *m, const void *image, unsigned n, unsigned *opts, void **vals);
  CUresult (*ModuleUnload)(CUmodule m);
  CUresult (*ModuleGetFunction)(CUfunction *f, CUmodule m, const char *name);
  CUresult (*ModuleGetGlobal)(CUdeviceptr *p, size_t *size, CUmodule m, const char *name);
  CUresult (*MemAlloc)(CUdeviceptr *p, size_t size);
  CUresult (*MemAllocHost)(void **p, size_t size);
  CUresult (*MemAllocManaged)(CUdeviceptr *p, size_t size, unsigned flags);
  CUresult (*MemFree)(CUdeviceptr p);
  CUresult (*MemFreeHost)(void *p);
  CUresult (*MemcpyHtoD)(CUdeviceptr dst, const void *src, size_t n);
  CUresult (*MemcpyDtoH)(void *dst, CUdeviceptr src, size_t n);
  CUresult (*MemcpyDtoD)(CUdeviceptr dst, CUdeviceptr src, size_t n);
  CUresult (*MemcpyHtoDAsync)(CUdeviceptr dst, const void *src, size_t n, CUstream s);
  CUresult (*MemcpyDtoHAsync)(void *dst, CUdeviceptr src, size_t n, CUstream s);
  CUresult (*MemcpyDtoDAsync)(CUdeviceptr dst, CUdeviceptr src, size_t n, CUstream s);
  CUresult (*MemsetD8)(CUdeviceptr p, unsigned char v, size_t n);
  CUresult (*LaunchKernel)(CUfunction f, unsigned gx, unsigned gy, unsigned gz, unsigned bx, unsigned by, unsigned bz,
                           unsigned shmem, CUstream s, void **params, void **extra);
  CUresult (*StreamCreate)(CUstream *s, unsigned flags);
  CUresult (*StreamDestroy)(CUstream s);
  CUresult (*StreamSynchronize)(CUstream s);
  CUresult (*EventCreate)(CUevent *e, unsigned flags);
  CUresult (*EventDestroy)(CUevent e);
  CUresult (*EventRecord)(CUevent e, CUstream s);
  CUresult (*EventSynchronize)(CUevent e);
  CUresult (*EventElapsedTime)(float *ms, CUevent a, CUevent b);
  CUresult (*GetErrorString)(CUresult r, const char **s);
} drv;

static void *driverSym(void *lib, const char *name) {
#if defined(_WIN32)
  return GetProcAddress(lib, name);
#else
  return dlsym(lib, name);
#endif
}

static int loadDriver(void) {
  if (drv.loaded) return drv.loaded > 0;
#if defined(_WIN32)
  void *lib = LoadLibraryA("nvcuda.dll");
#else
  void *lib = dlopen("libcuda.so.1", 2 /* RTLD_NOW */);
  if (!lib) lib = dlopen("libcuda.so", 2);
#endif
  if (!lib) {
    drv.loaded = -1;
    return 0;
  }
#define SYM(field, name) *(void **)&drv.field = driverSym(lib, name)
  SYM(Init, "cuInit");
  SYM(DeviceGetCount, "cuDeviceGetCount");
  SYM(DeviceGet, "cuDeviceGet");
  SYM(DeviceGetName, "cuDeviceGetName");
  SYM(DeviceGetAttribute, "cuDeviceGetAttribute");
  SYM(DeviceTotalMem, "cuDeviceTotalMem_v2");
  SYM(DevicePrimaryCtxRetain, "cuDevicePrimaryCtxRetain");
  SYM(CtxSetCurrent, "cuCtxSetCurrent");
  SYM(CtxSynchronize, "cuCtxSynchronize");
  SYM(ModuleLoadDataEx, "cuModuleLoadDataEx");
  SYM(ModuleUnload, "cuModuleUnload");
  SYM(ModuleGetFunction, "cuModuleGetFunction");
  SYM(ModuleGetGlobal, "cuModuleGetGlobal_v2");
  SYM(MemAlloc, "cuMemAlloc_v2");
  SYM(MemAllocHost, "cuMemAllocHost_v2");
  SYM(MemAllocManaged, "cuMemAllocManaged");
  SYM(MemFree, "cuMemFree_v2");
  SYM(MemFreeHost, "cuMemFreeHost");
  SYM(MemcpyHtoD, "cuMemcpyHtoD_v2");
  SYM(MemcpyDtoH, "cuMemcpyDtoH_v2");
  SYM(MemcpyDtoD, "cuMemcpyDtoD_v2");
  SYM(MemcpyHtoDAsync, "cuMemcpyHtoDAsync_v2");
  SYM(MemcpyDtoHAsync, "cuMemcpyDtoHAsync_v2");
  SYM(MemcpyDtoDAsync, "cuMemcpyDtoDAsync_v2");
  SYM(MemsetD8, "cuMemsetD8_v2");
  SYM(LaunchKernel, "cuLaunchKernel");
  SYM(StreamCreate, "cuStreamCreate");
  SYM(StreamDestroy, "cuStreamDestroy_v2");
  SYM(StreamSynchronize, "cuStreamSynchronize");
  SYM(EventCreate, "cuEventCreate");
  SYM(EventDestroy, "cuEventDestroy_v2");
  SYM(EventRecord, "cuEventRecord");
  SYM(EventSynchronize, "cuEventSynchronize");
  SYM(EventElapsedTime, "cuEventElapsedTime");
  SYM(GetErrorString, "cuGetErrorString");
#undef SYM
  drv.loaded = drv.Init && drv.LaunchKernel ? 1 : -1;
  return drv.loaded > 0;
}

/* ---- state -------------------------------------------------------------- */

static cudaError_t lastError = cudaSuccess;
static CUcontext context;
static CUdevice device;
static int deviceOrdinal;
static int contextReady;

// A registered fat binary and the module it loads to.
struct Fatbin {
  const void *wrapper; // what __cudaRegisterFatBinary was given
  CUmodule module;
  int loadFailed;
};

// A registered kernel: the stub's address and the device name.
struct Kernel {
  const void *hostFun;
  const char *deviceName;
  Fatbin *fatbin;
  CUfunction function;
};

// A registered __device__ or __constant__ object.
struct Var {
  const void *hostVar;
  const char *deviceName;
  Fatbin *fatbin;
  size_t size;
};

static Fatbin *fatbins[256];
static int nfatbins;
static Kernel kernels[4096];
static int nkernels;
static Var vars[4096];
static int nvars;

// The launch configurations pushed and not yet popped, one per nested
// stub call, which is never more than one deep in practice.
struct Config {
  unsigned gx, gy, gz, bx, by, bz;
  size_t shmem;
  cudaStream_t stream;
};
static Config configs[64];
static int nconfigs;

/* ---- errors ------------------------------------------------------------- */

static cudaError_t fromDriver(CUresult r) {
  switch (r) {
  case 0:
    return cudaSuccess;
  case 1:
    return cudaErrorInvalidValue;
  case 2:
    return cudaErrorMemoryAllocation;
  case 3:
    return cudaErrorInitializationError;
  case 100:
    return cudaErrorNoDevice;
  case 101:
    return cudaErrorInvalidDevice;
  case 500:
    return cudaErrorInvalidDeviceFunction;
  case 700:
  case 719:
    return cudaErrorLaunchFailure;
  }
  return cudaErrorUnknown;
}

static cudaError_t fail(cudaError_t e) {
  if (e != cudaSuccess) lastError = e;
  return e;
}

static cudaError_t check(CUresult r) { return fail(fromDriver(r)); }

/* ---- the context -------------------------------------------------------- */

static cudaError_t ensureContext(void) {
  if (contextReady) return cudaSuccess;
  if (!loadDriver()) return fail(cudaErrorNoDevice);
  CUresult r = drv.Init(0);
  if (r) return check(r);
  int count = 0;
  r = drv.DeviceGetCount(&count);
  if (r) return check(r);
  if (count <= 0 || deviceOrdinal >= count) return fail(cudaErrorNoDevice);
  r = drv.DeviceGet(&device, deviceOrdinal);
  if (r) return check(r);
  r = drv.DevicePrimaryCtxRetain(&context, device);
  if (r) return check(r);
  r = drv.CtxSetCurrent(context);
  if (r) return check(r);
  contextReady = 1;
  return cudaSuccess;
}

/* ---- fat binaries ------------------------------------------------------- */

// The fat binary container: a header, then entries each with a payload.
// The PTX entry is what the driver JITs; an ELF one for this device's
// architecture would be loaded as it is.
struct FatbinHeader {
  unsigned magic;
  unsigned short version;
  unsigned short headerSize;
  unsigned long long size;
};

struct FatbinEntry {
  unsigned short kind;
  unsigned short unknown1;
  unsigned headerSize;
  unsigned long long size;
  unsigned compressedSize;
  unsigned unknown2;
  unsigned short minor, major;
  unsigned arch;
  unsigned nameOffset, nameLen;
  unsigned long long flags;
  unsigned long long zero;
  unsigned long long decompressedSize;
};

struct FatbinWrapper {
  int magic;
  int version;
  const void *data;
  void *filename;
};

// findImage is the PTX text in a fat binary for the device: the newest
// architecture at or below the device's, or null.
static const char *findImage(const void *data, unsigned sm) {
  const FatbinHeader *h = (const FatbinHeader *)data;
  if (h->magic != 0xBA55ED50u) return 0;
  const char *p = (const char *)data + h->headerSize;
  const char *end = p + h->size;
  const char *best = 0;
  unsigned bestArch = 0;
  while (p + sizeof(FatbinEntry) <= end) {
    const FatbinEntry *e = (const FatbinEntry *)p;
    if (e->kind == 1 && e->arch <= sm && (best == 0 || e->arch > bestArch)) {
      best = p + e->headerSize;
      bestArch = e->arch;
    }
    p += e->headerSize + e->size;
  }
  return best;
}

static cudaError_t loadModule(Fatbin *fb) {
  if (fb->module) return cudaSuccess;
  if (fb->loadFailed) return fail(cudaErrorInvalidDeviceFunction);
  cudaError_t e = ensureContext();
  if (e) return e;
  int major = 0, minor = 0;
  drv.DeviceGetAttribute(&major, 75, device);
  drv.DeviceGetAttribute(&minor, 76, device);
  const FatbinWrapper *w = (const FatbinWrapper *)fb->wrapper;
  const char *image = w && w->magic == 0x466243B1 ? findImage(w->data, (unsigned)(major * 10 + minor)) : 0;
  if (!image) {
    fb->loadFailed = 1;
    fprintf(stderr, "vcx cudart: no device image for sm_%d%d in the fat binary\n", major, minor);
    return fail(cudaErrorInvalidDeviceFunction);
  }
  static char log[4096];
  unsigned opts[2] = {5 /* CU_JIT_ERROR_LOG_BUFFER */, 6 /* CU_JIT_ERROR_LOG_BUFFER_SIZE_BYTES */};
  void *vals[2] = {log, (void *)(size_t)sizeof log};
  log[0] = 0;
  CUresult r = drv.ModuleLoadDataEx(&fb->module, image, 2, opts, vals);
  if (r) {
    fb->loadFailed = 1;
    fprintf(stderr, "vcx cudart: the device image failed to load: %s\n", log);
    return check(r);
  }
  return cudaSuccess;
}

static Kernel *findKernel(const void *hostFun) {
  for (int i = 0; i < nkernels; i++)
    if (kernels[i].hostFun == hostFun) return &kernels[i];
  return 0;
}

static Var *findVar(const void *hostVar) {
  for (int i = 0; i < nvars; i++)
    if (vars[i].hostVar == hostVar) return &vars[i];
  return 0;
}

static cudaError_t functionOf(Kernel *k, CUfunction *out) {
  if (!k->function) {
    cudaError_t e = loadModule(k->fatbin);
    if (e) return e;
    CUresult r = drv.ModuleGetFunction(&k->function, k->fatbin->module, k->deviceName);
    if (r) return check(r);
  }
  *out = k->function;
  return cudaSuccess;
}

static cudaError_t addressOf(Var *v, CUdeviceptr *out, size_t *size) {
  cudaError_t e = loadModule(v->fatbin);
  if (e) return e;
  return check(drv.ModuleGetGlobal(out, size, v->fatbin->module, v->deviceName));
}

/* ---- the registration a unit's constructor makes ------------------------ */

extern "C" {

void **__cudaRegisterFatBinary(void *wrapper) {
  if (nfatbins >= 256) return 0;
  Fatbin *fb = (Fatbin *)calloc(1, sizeof(Fatbin));
  fb->wrapper = wrapper;
  fatbins[nfatbins++] = fb;
  return (void **)fb;
}

void __cudaRegisterFatBinaryEnd(void **handle) { (void)handle; }

void __cudaUnregisterFatBinary(void **handle) {
  Fatbin *fb = (Fatbin *)handle;
  if (fb && fb->module && drv.ModuleUnload) drv.ModuleUnload(fb->module);
  if (fb) fb->module = (CUmodule)0;
}

void __cudaRegisterFunction(void **handle, const char *hostFun, char *deviceFun, const char *deviceName,
                            int threadLimit, void *tid, void *bid, void *bDim, void *gDim, int *wSize) {
  (void)deviceFun; (void)threadLimit; (void)tid; (void)bid; (void)bDim; (void)gDim; (void)wSize;
  if (nkernels >= 4096) return;
  Kernel *k = &kernels[nkernels++];
  k->hostFun = hostFun;
  k->deviceName = deviceName;
  k->fatbin = (Fatbin *)handle;
  k->function = (CUfunction)0;
}

void __cudaRegisterVar(void **handle, char *hostVar, char *deviceAddress, const char *deviceName, int ext,
                       size_t size, int constant, int global) {
  (void)deviceAddress; (void)ext; (void)constant; (void)global;
  if (nvars >= 4096) return;
  Var *v = &vars[nvars++];
  v->hostVar = hostVar;
  v->deviceName = deviceName;
  v->fatbin = (Fatbin *)handle;
  v->size = size;
}

/* ---- launches ----------------------------------------------------------- */

unsigned __vcx_cudaPushCallConfiguration(unsigned gx, unsigned gy, unsigned gz, unsigned bx, unsigned by,
                                         unsigned bz, size_t shmem, void *stream) {
  if (nconfigs >= 64) return cudaErrorLaunchFailure;
  Config *c = &configs[nconfigs++];
  c->gx = gx; c->gy = gy; c->gz = gz;
  c->bx = bx; c->by = by; c->bz = bz;
  c->shmem = shmem;
  c->stream = (cudaStream_t)stream;
  return cudaSuccess;
}

cudaError_t __cudaPopCallConfiguration(dim3 *grid, dim3 *block, size_t *shmem, void **stream) {
  if (nconfigs == 0) return cudaErrorLaunchFailure;
  Config *c = &configs[--nconfigs];
  grid->x = c->gx; grid->y = c->gy; grid->z = c->gz;
  block->x = c->bx; block->y = c->by; block->z = c->bz;
  *shmem = c->shmem;
  *stream = c->stream;
  return cudaSuccess;
}

cudaError_t __vcx_cudaLaunch(const void *func, unsigned gx, unsigned gy, unsigned gz, unsigned bx, unsigned by,
                             unsigned bz, void **args, size_t shmem, cudaStream_t stream) {
  Kernel *k = findKernel(func);
  if (!k) return fail(cudaErrorInvalidDeviceFunction);
  CUfunction f;
  cudaError_t e = functionOf(k, &f);
  if (e) return e;
  return check(drv.LaunchKernel(f, gx, gy, gz, bx, by, bz, (unsigned)shmem, (CUstream)stream, args, 0));
}

cudaError_t cudaLaunchKernel(const void *func, dim3 grid, dim3 block, void **args, size_t shmem, cudaStream_t stream) {
  return __vcx_cudaLaunch(func, grid.x, grid.y, grid.z, block.x, block.y, block.z, args, shmem, stream);
}

/* ---- memory --------------------------------------------------------------- */

cudaError_t cudaMalloc(void **devPtr, size_t size) {
  cudaError_t e = ensureContext();
  if (e) return e;
  CUdeviceptr p = 0;
  e = check(drv.MemAlloc(&p, size ? size : 1));
  *devPtr = (void *)(size_t)p;
  return e;
}

cudaError_t cudaFree(void *devPtr) {
  if (!devPtr) return cudaSuccess;
  cudaError_t e = ensureContext();
  if (e) return e;
  return check(drv.MemFree((CUdeviceptr)(size_t)devPtr));
}

cudaError_t cudaMallocHost(void **ptr, size_t size) {
  cudaError_t e = ensureContext();
  if (e) return e;
  return check(drv.MemAllocHost(ptr, size ? size : 1));
}

cudaError_t cudaFreeHost(void *ptr) {
  if (!ptr) return cudaSuccess;
  cudaError_t e = ensureContext();
  if (e) return e;
  return check(drv.MemFreeHost(ptr));
}

cudaError_t cudaMallocManaged(void **devPtr, size_t size, unsigned flags) {
  cudaError_t e = ensureContext();
  if (e) return e;
  CUdeviceptr p = 0;
  e = check(drv.MemAllocManaged(&p, size ? size : 1, flags ? flags : 1 /* CU_MEM_ATTACH_GLOBAL */));
  *devPtr = (void *)(size_t)p;
  return e;
}

cudaError_t cudaMemcpy(void *dst, const void *src, size_t count, cudaMemcpyKind kind) {
  cudaError_t e = ensureContext();
  if (e) return e;
  switch (kind) {
  case cudaMemcpyHostToDevice:
    return check(drv.MemcpyHtoD((CUdeviceptr)(size_t)dst, src, count));
  case cudaMemcpyDeviceToHost:
    return check(drv.MemcpyDtoH(dst, (CUdeviceptr)(size_t)src, count));
  case cudaMemcpyDeviceToDevice:
  case cudaMemcpyDefault:
    return check(drv.MemcpyDtoD((CUdeviceptr)(size_t)dst, (CUdeviceptr)(size_t)src, count));
  case cudaMemcpyHostToHost:
    memcpy(dst, src, count);
    return cudaSuccess;
  }
  return fail(cudaErrorInvalidValue);
}

cudaError_t cudaMemcpyAsync(void *dst, const void *src, size_t count, cudaMemcpyKind kind, cudaStream_t stream) {
  cudaError_t e = ensureContext();
  if (e) return e;
  switch (kind) {
  case cudaMemcpyHostToDevice:
    return check(drv.MemcpyHtoDAsync((CUdeviceptr)(size_t)dst, src, count, (CUstream)stream));
  case cudaMemcpyDeviceToHost:
    return check(drv.MemcpyDtoHAsync(dst, (CUdeviceptr)(size_t)src, count, (CUstream)stream));
  case cudaMemcpyDeviceToDevice:
  case cudaMemcpyDefault:
    return check(drv.MemcpyDtoDAsync((CUdeviceptr)(size_t)dst, (CUdeviceptr)(size_t)src, count, (CUstream)stream));
  case cudaMemcpyHostToHost:
    memcpy(dst, src, count);
    return cudaSuccess;
  }
  return fail(cudaErrorInvalidValue);
}

cudaError_t cudaMemset(void *devPtr, int value, size_t count) {
  cudaError_t e = ensureContext();
  if (e) return e;
  return check(drv.MemsetD8((CUdeviceptr)(size_t)devPtr, (unsigned char)value, count));
}

cudaError_t cudaMemcpyToSymbol(const void *symbol, const void *src, size_t count, size_t offset, cudaMemcpyKind kind) {
  Var *v = findVar(symbol);
  if (!v) return fail(cudaErrorInvalidValue);
  CUdeviceptr p;
  size_t size;
  cudaError_t e = addressOf(v, &p, &size);
  if (e) return e;
  if (kind == cudaMemcpyDeviceToDevice) return check(drv.MemcpyDtoD(p + offset, (CUdeviceptr)(size_t)src, count));
  return check(drv.MemcpyHtoD(p + offset, src, count));
}

cudaError_t cudaMemcpyFromSymbol(void *dst, const void *symbol, size_t count, size_t offset, cudaMemcpyKind kind) {
  Var *v = findVar(symbol);
  if (!v) return fail(cudaErrorInvalidValue);
  CUdeviceptr p;
  size_t size;
  cudaError_t e = addressOf(v, &p, &size);
  if (e) return e;
  if (kind == cudaMemcpyDeviceToDevice) return check(drv.MemcpyDtoD((CUdeviceptr)(size_t)dst, p + offset, count));
  return check(drv.MemcpyDtoH(dst, p + offset, count));
}

cudaError_t cudaGetSymbolAddress(void **devPtr, const void *symbol) {
  Var *v = findVar(symbol);
  if (!v) return fail(cudaErrorInvalidValue);
  CUdeviceptr p;
  size_t size;
  cudaError_t e = addressOf(v, &p, &size);
  if (e) return e;
  *devPtr = (void *)(size_t)p;
  return cudaSuccess;
}

cudaError_t cudaGetSymbolSize(size_t *size, const void *symbol) {
  Var *v = findVar(symbol);
  if (!v) return fail(cudaErrorInvalidValue);
  CUdeviceptr p;
  return addressOf(v, &p, size);
}

/* ---- devices and errors --------------------------------------------------- */

cudaError_t cudaDeviceSynchronize(void) {
  cudaError_t e = ensureContext();
  if (e) return e;
  return check(drv.CtxSynchronize());
}

cudaError_t cudaDeviceReset(void) { return cudaDeviceSynchronize(); }

cudaError_t cudaGetLastError(void) {
  cudaError_t e = lastError;
  lastError = cudaSuccess;
  return e;
}

cudaError_t cudaPeekAtLastError(void) { return lastError; }

const char *cudaGetErrorString(cudaError_t error) {
  switch (error) {
  case cudaSuccess:
    return "no error";
  case cudaErrorInvalidValue:
    return "invalid argument";
  case cudaErrorMemoryAllocation:
    return "out of memory";
  case cudaErrorInitializationError:
    return "initialization error";
  case cudaErrorInvalidConfiguration:
    return "invalid configuration argument";
  case cudaErrorInvalidDevice:
    return "invalid device ordinal";
  case cudaErrorNoDevice:
    return "no CUDA-capable device is detected";
  case cudaErrorInvalidDeviceFunction:
    return "invalid device function";
  case cudaErrorLaunchFailure:
    return "unspecified launch failure";
  default:
    return "unknown error";
  }
}

const char *cudaGetErrorName(cudaError_t error) {
  switch (error) {
  case cudaSuccess:
    return "cudaSuccess";
  case cudaErrorInvalidValue:
    return "cudaErrorInvalidValue";
  case cudaErrorMemoryAllocation:
    return "cudaErrorMemoryAllocation";
  case cudaErrorInitializationError:
    return "cudaErrorInitializationError";
  case cudaErrorInvalidConfiguration:
    return "cudaErrorInvalidConfiguration";
  case cudaErrorInvalidDevice:
    return "cudaErrorInvalidDevice";
  case cudaErrorNoDevice:
    return "cudaErrorNoDevice";
  case cudaErrorInvalidDeviceFunction:
    return "cudaErrorInvalidDeviceFunction";
  case cudaErrorLaunchFailure:
    return "cudaErrorLaunchFailure";
  default:
    return "cudaErrorUnknown";
  }
}

cudaError_t cudaGetDeviceCount(int *count) {
  *count = 0;
  if (!loadDriver()) return fail(cudaErrorNoDevice);
  CUresult r = drv.Init(0);
  if (r) return check(r);
  return check(drv.DeviceGetCount(count));
}

cudaError_t cudaGetDevice(int *dev) {
  *dev = deviceOrdinal;
  return cudaSuccess;
}

cudaError_t cudaSetDevice(int dev) {
  if (dev == deviceOrdinal) return cudaSuccess;
  if (contextReady) return fail(cudaErrorInvalidDevice);
  deviceOrdinal = dev;
  return cudaSuccess;
}

cudaError_t cudaGetDeviceProperties(struct cudaDeviceProp *prop, int dev) {
  if (!loadDriver()) return fail(cudaErrorNoDevice);
  CUresult r = drv.Init(0);
  if (r) return check(r);
  CUdevice d;
  r = drv.DeviceGet(&d, dev);
  if (r) return check(r);
  memset(prop, 0, sizeof *prop);
  drv.DeviceGetName(prop->name, sizeof prop->name, d);
  drv.DeviceTotalMem(&prop->totalGlobalMem, d);
  int v = 0;
  drv.DeviceGetAttribute(&v, 75, d);
  prop->major = v;
  drv.DeviceGetAttribute(&v, 76, d);
  prop->minor = v;
  drv.DeviceGetAttribute(&v, 1, d);
  prop->maxThreadsPerBlock = v;
  drv.DeviceGetAttribute(&v, 8, d);
  prop->sharedMemPerBlock = (size_t)v;
  drv.DeviceGetAttribute(&v, 10, d);
  prop->warpSize = v;
  drv.DeviceGetAttribute(&v, 16, d);
  prop->multiProcessorCount = v;
  drv.DeviceGetAttribute(&v, 13, d);
  prop->clockRate = v;
  drv.DeviceGetAttribute(&v, 12, d);
  prop->regsPerBlock = v;
  drv.DeviceGetAttribute(&v, 9, d);
  prop->totalConstMem = (size_t)v;
  for (int i = 0; i < 3; i++) {
    drv.DeviceGetAttribute(&v, 2 + i, d);
    prop->maxThreadsDim[i] = v;
    drv.DeviceGetAttribute(&v, 5 + i, d);
    prop->maxGridSize[i] = v;
  }
  return cudaSuccess;
}

/* ---- streams and events --------------------------------------------------- */

cudaError_t cudaStreamCreate(cudaStream_t *stream) {
  cudaError_t e = ensureContext();
  if (e) return e;
  return check(drv.StreamCreate((CUstream *)stream, 0));
}

cudaError_t cudaStreamDestroy(cudaStream_t stream) {
  if (!stream) return cudaSuccess;
  return check(drv.StreamDestroy((CUstream)stream));
}

cudaError_t cudaStreamSynchronize(cudaStream_t stream) {
  cudaError_t e = ensureContext();
  if (e) return e;
  if (!stream) return check(drv.CtxSynchronize());
  return check(drv.StreamSynchronize((CUstream)stream));
}

cudaError_t cudaEventCreate(cudaEvent_t *event) {
  cudaError_t e = ensureContext();
  if (e) return e;
  return check(drv.EventCreate((CUevent *)event, 0));
}

cudaError_t cudaEventDestroy(cudaEvent_t event) { return check(drv.EventDestroy((CUevent)event)); }

cudaError_t cudaEventRecord(cudaEvent_t event, cudaStream_t stream) {
  return check(drv.EventRecord((CUevent)event, (CUstream)stream));
}

cudaError_t cudaEventSynchronize(cudaEvent_t event) { return check(drv.EventSynchronize((CUevent)event)); }

cudaError_t cudaEventElapsedTime(float *ms, cudaEvent_t start, cudaEvent_t end) {
  return check(drv.EventElapsedTime(ms, (CUevent)start, (CUevent)end));
}

} /* extern "C" */
