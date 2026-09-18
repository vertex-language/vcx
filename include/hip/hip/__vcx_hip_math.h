/*
 * __vcx_hip_math.h -- the device math library's surface, for HIP.
 *
 * The C math functions are declared here with their C names, so that a
 * kernel may call sqrtf without including <cmath>; the host's <cmath>,
 * read later, redeclares the same functions and adds nothing. In the
 * device pass a call to any of them is the device library's: the ones
 * the hardware computes in an instruction -- a square root, a floor, a
 * fused multiply-add, the fast approximations -- lower to VIR's verbs,
 * and the rest are not lowered yet and say so by name.
 *
 * The __-prefixed intrinsics are the approximate forms, over the
 * __builtin_amdgcn_* builtins that name the instruction.
 */
#ifndef __VCX_HIP_MATH_H__
#define __VCX_HIP_MATH_H__


extern "C" {
__host__ __device__ float sqrtf(float);
__host__ __device__ double sqrt(double);
__host__ __device__ float fabsf(float);
__host__ __device__ double fabs(double);
__host__ __device__ float floorf(float);
__host__ __device__ double floor(double);
__host__ __device__ float ceilf(float);
__host__ __device__ double ceil(double);
__host__ __device__ float truncf(float);
__host__ __device__ double trunc(double);
__host__ __device__ float rintf(float);
__host__ __device__ double rint(double);
__host__ __device__ float nearbyintf(float);
__host__ __device__ double nearbyint(double);
__host__ __device__ float roundf(float);
__host__ __device__ double round(double);
__host__ __device__ float fminf(float, float);
__host__ __device__ double fmin(double, double);
__host__ __device__ float fmaxf(float, float);
__host__ __device__ double fmax(double, double);
__host__ __device__ float fmaf(float, float, float);
__host__ __device__ double fma(double, double, double);
__host__ __device__ float copysignf(float, float);
__host__ __device__ double copysign(double, double);
__host__ __device__ float rsqrtf(float);
__host__ __device__ double rsqrt(double);
__host__ __device__ float expf(float);
__host__ __device__ double exp(double);
__host__ __device__ float exp2f(float);
__host__ __device__ double exp2(double);
__host__ __device__ float logf(float);
__host__ __device__ double log(double);
__host__ __device__ float log2f(float);
__host__ __device__ double log2(double);
__host__ __device__ float sinf(float);
__host__ __device__ double sin(double);
__host__ __device__ float cosf(float);
__host__ __device__ double cos(double);
__host__ __device__ float tanf(float);
__host__ __device__ double tan(double);
__host__ __device__ float powf(float, float);
__host__ __device__ double pow(double, double);
__host__ __device__ float exp10f(float);
__host__ __device__ double exp10(double);
__host__ __device__ float log10f(float);
__host__ __device__ double log10(double);
__host__ __device__ void sincosf(float, float *, float *);
__host__ __device__ void sincos(double, double *, double *);
__host__ __device__ float fmodf(float, float);
__host__ __device__ double fmod(double, double);
__host__ __device__ float sinhf(float);
__host__ __device__ double sinh(double);
__host__ __device__ float coshf(float);
__host__ __device__ double cosh(double);
__host__ __device__ float tanhf(float);
__host__ __device__ double tanh(double);
__host__ __device__ float atanf(float);
__host__ __device__ double atan(double);
__host__ __device__ float atan2f(float, float);
__host__ __device__ double atan2(double, double);
__host__ __device__ float asinf(float);
__host__ __device__ double asin(double);
__host__ __device__ float acosf(float);
__host__ __device__ double acos(double);
__host__ __device__ float ldexpf(float, int);
__host__ __device__ double ldexp(double, int);
}

/* The fast, approximate forms, over the AMD instructions. */
__VCX_DEVICE_INLINE float __fsqrt_rn(float x) { return __builtin_amdgcn_sqrtf(x); }
__VCX_DEVICE_INLINE float __frsqrt_rn(float x) { return __builtin_amdgcn_rsqf(x); }
__VCX_DEVICE_INLINE float __frcp_rn(float x) { return __builtin_amdgcn_rcpf(x); }
__VCX_DEVICE_INLINE float __fdividef(float a, float b) { return a * __builtin_amdgcn_rcpf(b); }
__VCX_DEVICE_INLINE float __exp2f(float x) { return __builtin_amdgcn_exp2f(x); }
__VCX_DEVICE_INLINE float __expf(float x) { return __builtin_amdgcn_exp2f(x * 1.4426950408889634f); }
__VCX_DEVICE_INLINE float __log2f(float x) { return __builtin_amdgcn_logf(x); }
__VCX_DEVICE_INLINE float __logf(float x) { return __builtin_amdgcn_logf(x) * 0.69314718055994531f; }
__VCX_DEVICE_INLINE float __log10f(float x) { return __builtin_amdgcn_logf(x) * 0.30102999566398120f; }
__VCX_DEVICE_INLINE float __sinf(float x) { return __builtin_amdgcn_sinf(x * 0.15915494309189535f); }
__VCX_DEVICE_INLINE float __cosf(float x) { return __builtin_amdgcn_cosf(x * 0.15915494309189535f); }
__VCX_DEVICE_INLINE float __powf(float a, float b) { return __builtin_amdgcn_exp2f(b * __builtin_amdgcn_logf(a)); }
__VCX_DEVICE_INLINE float __fmaf_rn(float a, float b, float c) { return fmaf(a, b, c); }
__VCX_DEVICE_INLINE double __fma_rn(double a, double b, double c) { return fma(a, b, c); }
__VCX_DEVICE_INLINE float __fadd_rn(float a, float b) { return a + b; }
__VCX_DEVICE_INLINE float __fsub_rn(float a, float b) { return a - b; }
__VCX_DEVICE_INLINE float __fmul_rn(float a, float b) { return a * b; }
__VCX_DEVICE_INLINE float __fdiv_rn(float a, float b) { return a / b; }
__VCX_DEVICE_INLINE float __saturatef(float x) { return x < 0.0f ? 0.0f : x > 1.0f ? 1.0f : x; }

#endif /* __VCX_HIP_MATH_H__ */
