/*
 * __vcx_devicelib_math.h -- the device math library, in C.
 *
 * What the hardware has no instruction for: the exponentials, the
 * logarithms, the trigonometric functions and pow, in float and double.
 * Each is a __device__ function named __vcx_<name>, and the device pass
 * routes a call to the C name -- expf, sin, pow -- to it, so that a
 * kernel calling expf reaches this whether it included <cmath> or not.
 * The same file serves CUDA and HIP: it is written over the C operators
 * and the single-instruction functions the compiler lowers by name.
 *
 * The algorithms are the classic ones -- Cody-Waite reduction and a
 * minimax or Taylor polynomial -- and their accuracy is a few ulp, not
 * the correctly-rounded results of a full libdevice or ocml: sinf and
 * cosf hold to about 1 ulp for |x| below 1e5 and degrade past it, the
 * double sin and cos likewise past 1e9; expf and logf are within 2 ulp;
 * pow is exp of a log and inherits both errors. That is the state of the
 * library, not its ambition.
 */
#ifndef __VCX_DEVICELIB_MATH_H__
#define __VCX_DEVICELIB_MATH_H__

/* ---- float -------------------------------------------------------------- */

__VCX_DEVICE_INLINE float __vcx_ldexpf(float x, int e) {
  /* Two steps keep the scale factor representable for |e| up to 254. */
  if (e > 127) { x *= 1.7014118346046923e38f; e -= 127; }
  if (e < -126) { x *= 1.1754943508222875e-38f; e += 126; }
  if (e > 127) e = 127;
  if (e < -126) e = -126;
  union { unsigned u; float f; } s;
  s.u = (unsigned)(e + 127) << 23;
  return x * s.f;
}

__VCX_DEVICE_INLINE float __vcx_expf(float x) {
  if (x > 88.72283f) return __builtin_huge_valf();
  if (x < -103.97f) return 0.0f;
  float k = rintf(x * 1.44269504088896341f);
  float r = fmaf(k, -0.693145751953125f, x);
  r = fmaf(k, -1.428606765330187e-06f, r);
  /* e^r on [-ln2/2, ln2/2]: a degree-6 Taylor polynomial, Horner form. */
  float p = 1.0f / 720.0f;
  p = fmaf(p, r, 1.0f / 120.0f);
  p = fmaf(p, r, 1.0f / 24.0f);
  p = fmaf(p, r, 1.0f / 6.0f);
  p = fmaf(p, r, 0.5f);
  p = fmaf(p, r, 1.0f);
  p = fmaf(p, r, 1.0f);
  return __vcx_ldexpf(p, (int)k);
}

__VCX_DEVICE_INLINE float __vcx_exp2f(float x) {
  if (x > 128.0f) return __builtin_huge_valf();
  if (x < -150.0f) return 0.0f;
  float k = rintf(x);
  float r = (x - k) * 0.693147180559945309f;
  float p = 1.0f / 720.0f;
  p = fmaf(p, r, 1.0f / 120.0f);
  p = fmaf(p, r, 1.0f / 24.0f);
  p = fmaf(p, r, 1.0f / 6.0f);
  p = fmaf(p, r, 0.5f);
  p = fmaf(p, r, 1.0f);
  p = fmaf(p, r, 1.0f);
  return __vcx_ldexpf(p, (int)k);
}

__VCX_DEVICE_INLINE float __vcx_exp10f(float x) { return __vcx_expf(x * 2.302585092994046f); }

/* logf: x = m * 2^e with m in [sqrt(1/2), sqrt(2)); log(m) = 2 atanh(s),
 * s = (m - 1)/(m + 1), as a polynomial in s^2. */
__VCX_DEVICE_INLINE float __vcx_logf(float x) {
  if (x < 0.0f || x != x) return __builtin_nanf("");
  if (x == 0.0f) return -__builtin_huge_valf();
  if (x == __builtin_huge_valf()) return x;
  union { float f; unsigned u; } v;
  v.f = x;
  int e = 0;
  if (v.u < 0x00800000u) { /* subnormal */
    v.f *= 8388608.0f;
    e = -23;
  }
  e += (int)(v.u >> 23) - 127;
  v.u = (v.u & 0x007fffffu) | 0x3f800000u; /* m in [1, 2) */
  float m = v.f;
  if (m > 1.41421356f) { m *= 0.5f; e += 1; }
  float s = (m - 1.0f) / (m + 1.0f);
  float z = s * s;
  float p = 1.0f / 9.0f;
  p = fmaf(p, z, 1.0f / 7.0f);
  p = fmaf(p, z, 1.0f / 5.0f);
  p = fmaf(p, z, 1.0f / 3.0f);
  p = fmaf(p, z, 1.0f);
  return fmaf((float)e, 0.693147180559945309f, 2.0f * s * p);
}

__VCX_DEVICE_INLINE float __vcx_log2f(float x) { return __vcx_logf(x) * 1.44269504088896341f; }
__VCX_DEVICE_INLINE float __vcx_log10f(float x) { return __vcx_logf(x) * 0.434294481903251828f; }

/* sinf and cosf: reduce by pi/2 in three parts, then the cephes
 * polynomials on [-pi/4, pi/4]. */
__VCX_DEVICE_INLINE float __vcx_sincosf_reduce(float x, int *quadrant) {
  float j = rintf(x * 0.636619772367581343f);
  *quadrant = (int)j & 3;
  float r = fmaf(j, -1.5703125f, x);
  r = fmaf(j, -4.837512969970703125e-4f, r);
  r = fmaf(j, -7.549789948768648e-8f, r);
  return r;
}

__VCX_DEVICE_INLINE float __vcx_sinf_poly(float r) {
  float z = r * r;
  float p = -1.9515295891e-4f;
  p = fmaf(p, z, 8.3321608736e-3f);
  p = fmaf(p, z, -1.6666654611e-1f);
  return fmaf(p * z, r, r);
}

__VCX_DEVICE_INLINE float __vcx_cosf_poly(float r) {
  float z = r * r;
  float p = 2.443315711809948e-5f;
  p = fmaf(p, z, -1.388731625493765e-3f);
  p = fmaf(p, z, 4.166664568298827e-2f);
  return fmaf(p * z, z, fmaf(-0.5f, z, 1.0f));
}

__VCX_DEVICE_INLINE float __vcx_sinf(float x) {
  if (x != x || x == __builtin_huge_valf() || x == -__builtin_huge_valf()) return __builtin_nanf("");
  int q;
  float r = __vcx_sincosf_reduce(x, &q);
  float s = (q & 1) ? __vcx_cosf_poly(r) : __vcx_sinf_poly(r);
  return (q & 2) ? -s : s;
}

__VCX_DEVICE_INLINE float __vcx_cosf(float x) {
  if (x != x || x == __builtin_huge_valf() || x == -__builtin_huge_valf()) return __builtin_nanf("");
  int q;
  float r = __vcx_sincosf_reduce(x, &q);
  float c = (q & 1) ? __vcx_sinf_poly(r) : __vcx_cosf_poly(r);
  return ((q + 1) & 2) ? -c : c;
}

__VCX_DEVICE_INLINE float __vcx_tanf(float x) { return __vcx_sinf(x) / __vcx_cosf(x); }

__VCX_DEVICE_INLINE void __vcx_sincosf(float x, float *s, float *c) {
  *s = __vcx_sinf(x);
  *c = __vcx_cosf(x);
}

__VCX_DEVICE_INLINE float __vcx_powf(float a, float b) {
  if (b == 0.0f) return 1.0f;
  if (a == 1.0f) return 1.0f;
  if (a != a || b != b) return a + b;
  if (a == 0.0f) return (b < 0.0f) ? __builtin_huge_valf() : 0.0f;
  if (a < 0.0f) {
    float bi = rintf(b);
    if (bi != b) return __builtin_nanf("");
    float r = __vcx_expf(b * __vcx_logf(-a));
    return (fmodf(bi, 2.0f) != 0.0f) ? -r : r;
  }
  return __vcx_expf(b * __vcx_logf(a));
}

__VCX_DEVICE_INLINE float __vcx_fmodf(float a, float b) {
  if (b == 0.0f || a != a || b != b) return __builtin_nanf("");
  float q = truncf(a / b);
  return fmaf(-q, b, a);
}

__VCX_DEVICE_INLINE float __vcx_sinhf(float x) { float e = __vcx_expf(x); return 0.5f * (e - 1.0f / e); }
__VCX_DEVICE_INLINE float __vcx_coshf(float x) { float e = __vcx_expf(x); return 0.5f * (e + 1.0f / e); }
__VCX_DEVICE_INLINE float __vcx_tanhf(float x) {
  if (x > 9.0f) return 1.0f;
  if (x < -9.0f) return -1.0f;
  float e = __vcx_expf(2.0f * x);
  return (e - 1.0f) / (e + 1.0f);
}

__VCX_DEVICE_INLINE float __vcx_atanf(float x) {
  /* Reduce to [0, 1] then to [0, tan(pi/8)] by atan(x) = pi/4 + atan((x-1)/(x+1)). */
  float sign = 1.0f;
  if (x < 0.0f) { x = -x; sign = -1.0f; }
  float add = 0.0f;
  if (x > 2.414213562373095f) { add = 1.5707963267948966f; x = -1.0f / x; }
  else if (x > 0.4142135623730950f) { add = 0.7853981633974483f; x = (x - 1.0f) / (x + 1.0f); }
  float z = x * x;
  float p = 8.05374449538e-2f;
  p = fmaf(p, z, -1.38776856032e-1f);
  p = fmaf(p, z, 1.99777106478e-1f);
  p = fmaf(p, z, -3.33329491539e-1f);
  return sign * (fmaf(p * z, x, x) + add);
}

__VCX_DEVICE_INLINE float __vcx_atan2f(float y, float x) {
  if (x > 0.0f) return __vcx_atanf(y / x);
  if (x < 0.0f) return __vcx_atanf(y / x) + ((y < 0.0f) ? -3.14159265358979f : 3.14159265358979f);
  if (y > 0.0f) return 1.5707963267948966f;
  if (y < 0.0f) return -1.5707963267948966f;
  return 0.0f;
}

__VCX_DEVICE_INLINE float __vcx_asinf(float x) { return __vcx_atan2f(x, sqrtf(fmaf(-x, x, 1.0f))); }
__VCX_DEVICE_INLINE float __vcx_acosf(float x) { return __vcx_atan2f(sqrtf(fmaf(-x, x, 1.0f)), x); }

/* ---- double --------------------------------------------------------------- */

__VCX_DEVICE_INLINE double __vcx_ldexp(double x, int e) {
  if (e > 1023) { x *= 8.98846567431158e307; e -= 1023; }
  if (e < -1022) { x *= 2.2250738585072014e-308; e += 1022; }
  if (e > 1023) e = 1023;
  if (e < -1022) e = -1022;
  union { unsigned long long u; double f; } s;
  s.u = (unsigned long long)(e + 1023) << 52;
  return x * s.f;
}

__VCX_DEVICE_INLINE double __vcx_exp(double x) {
  if (x > 709.782712893384) return __builtin_huge_val();
  if (x < -745.2) return 0.0;
  double k = rint(x * 1.4426950408889634074);
  double r = fma(k, -0.693147180369123816490, x);
  r = fma(k, -1.90821492927058770002e-10, r);
  /* e^r on [-ln2/2, ln2/2]: Taylor to r^12. */
  double p = 1.0 / 479001600.0;
  p = fma(p, r, 1.0 / 39916800.0);
  p = fma(p, r, 1.0 / 3628800.0);
  p = fma(p, r, 1.0 / 362880.0);
  p = fma(p, r, 1.0 / 40320.0);
  p = fma(p, r, 1.0 / 5040.0);
  p = fma(p, r, 1.0 / 720.0);
  p = fma(p, r, 1.0 / 120.0);
  p = fma(p, r, 1.0 / 24.0);
  p = fma(p, r, 1.0 / 6.0);
  p = fma(p, r, 0.5);
  p = fma(p, r, 1.0);
  p = fma(p, r, 1.0);
  return __vcx_ldexp(p, (int)k);
}

__VCX_DEVICE_INLINE double __vcx_exp2(double x) { return __vcx_exp(x * 0.69314718055994530942); }
__VCX_DEVICE_INLINE double __vcx_exp10(double x) { return __vcx_exp(x * 2.30258509299404568402); }

__VCX_DEVICE_INLINE double __vcx_log(double x) {
  if (x < 0.0 || x != x) return __builtin_nan("");
  if (x == 0.0) return -__builtin_huge_val();
  if (x == __builtin_huge_val()) return x;
  union { double f; unsigned long long u; } v;
  v.f = x;
  int e = 0;
  if (v.u < 0x0010000000000000ull) {
    v.f *= 18014398509481984.0; /* 2^54 */
    e = -54;
  }
  e += (int)(v.u >> 52) - 1023;
  v.u = (v.u & 0x000fffffffffffffull) | 0x3ff0000000000000ull;
  double m = v.f;
  if (m > 1.4142135623730951) { m *= 0.5; e += 1; }
  double s = (m - 1.0) / (m + 1.0);
  double z = s * s;
  /* 2 atanh(s) = 2 (s + s^3/3 + ... + s^21/21); |s| <= 0.1716. */
  double p = 1.0 / 21.0;
  p = fma(p, z, 1.0 / 19.0);
  p = fma(p, z, 1.0 / 17.0);
  p = fma(p, z, 1.0 / 15.0);
  p = fma(p, z, 1.0 / 13.0);
  p = fma(p, z, 1.0 / 11.0);
  p = fma(p, z, 1.0 / 9.0);
  p = fma(p, z, 1.0 / 7.0);
  p = fma(p, z, 1.0 / 5.0);
  p = fma(p, z, 1.0 / 3.0);
  p = fma(p, z, 1.0);
  return fma((double)e, 0.69314718055994530942, 2.0 * s * p);
}

__VCX_DEVICE_INLINE double __vcx_log2(double x) { return __vcx_log(x) * 1.4426950408889634074; }
__VCX_DEVICE_INLINE double __vcx_log10(double x) { return __vcx_log(x) * 0.43429448190325182765; }

__VCX_DEVICE_INLINE double __vcx_sincos_reduce(double x, int *quadrant) {
  double j = rint(x * 0.63661977236758134308);
  *quadrant = (int)j & 3;
  double r = fma(j, -1.57079632673412561417, x);
  r = fma(j, -6.07710050650619224932e-11, r);
  r = fma(j, -2.02226624879595063154e-21, r);
  return r;
}

__VCX_DEVICE_INLINE double __vcx_sin_poly(double r) {
  double z = r * r;
  /* r - r^3/6 + ... - r^17/17! */
  double p = -1.0 / 355687428096000.0;
  p = fma(p, z, 1.0 / 1307674368000.0);
  p = fma(p, z, -1.0 / 6227020800.0);
  p = fma(p, z, 1.0 / 39916800.0);
  p = fma(p, z, -1.0 / 362880.0);
  p = fma(p, z, 1.0 / 5040.0);
  p = fma(p, z, -1.0 / 120.0);
  p = fma(p, z, 1.0 / 6.0);
  return fma(-p * z, r, r);
}

__VCX_DEVICE_INLINE double __vcx_cos_poly(double r) {
  double z = r * r;
  /* 1 - r^2/2 + ... + r^16/16! */
  double p = 1.0 / 20922789888000.0;
  p = fma(p, z, -1.0 / 87178291200.0);
  p = fma(p, z, 1.0 / 479001600.0);
  p = fma(p, z, -1.0 / 3628800.0);
  p = fma(p, z, 1.0 / 40320.0);
  p = fma(p, z, -1.0 / 720.0);
  p = fma(p, z, 1.0 / 24.0);
  p = fma(p, z, -0.5);
  return fma(p, z, 1.0);
}

__VCX_DEVICE_INLINE double __vcx_sin(double x) {
  if (x != x || x == __builtin_huge_val() || x == -__builtin_huge_val()) return __builtin_nan("");
  int q;
  double r = __vcx_sincos_reduce(x, &q);
  double s = (q & 1) ? __vcx_cos_poly(r) : __vcx_sin_poly(r);
  return (q & 2) ? -s : s;
}

__VCX_DEVICE_INLINE double __vcx_cos(double x) {
  if (x != x || x == __builtin_huge_val() || x == -__builtin_huge_val()) return __builtin_nan("");
  int q;
  double r = __vcx_sincos_reduce(x, &q);
  double c = (q & 1) ? __vcx_sin_poly(r) : __vcx_cos_poly(r);
  return ((q + 1) & 2) ? -c : c;
}

__VCX_DEVICE_INLINE double __vcx_tan(double x) { return __vcx_sin(x) / __vcx_cos(x); }

__VCX_DEVICE_INLINE void __vcx_sincos(double x, double *s, double *c) {
  *s = __vcx_sin(x);
  *c = __vcx_cos(x);
}

__VCX_DEVICE_INLINE double __vcx_fmod(double a, double b) {
  if (b == 0.0 || a != a || b != b) return __builtin_nan("");
  double q = trunc(a / b);
  return fma(-q, b, a);
}

__VCX_DEVICE_INLINE double __vcx_pow(double a, double b) {
  if (b == 0.0) return 1.0;
  if (a == 1.0) return 1.0;
  if (a != a || b != b) return a + b;
  if (a == 0.0) return (b < 0.0) ? __builtin_huge_val() : 0.0;
  if (a < 0.0) {
    double bi = rint(b);
    if (bi != b) return __builtin_nan("");
    double r = __vcx_exp(b * __vcx_log(-a));
    return (__vcx_fmod(bi, 2.0) != 0.0) ? -r : r;
  }
  return __vcx_exp(b * __vcx_log(a));
}

__VCX_DEVICE_INLINE double __vcx_sinh(double x) { double e = __vcx_exp(x); return 0.5 * (e - 1.0 / e); }
__VCX_DEVICE_INLINE double __vcx_cosh(double x) { double e = __vcx_exp(x); return 0.5 * (e + 1.0 / e); }
__VCX_DEVICE_INLINE double __vcx_tanh(double x) {
  if (x > 20.0) return 1.0;
  if (x < -20.0) return -1.0;
  double e = __vcx_exp(2.0 * x);
  return (e - 1.0) / (e + 1.0);
}

__VCX_DEVICE_INLINE double __vcx_atan(double x) {
  double sign = 1.0;
  if (x < 0.0) { x = -x; sign = -1.0; }
  double add = 0.0;
  if (x > 2.4142135623730950488) { add = 1.5707963267948966192; x = -1.0 / x; }
  else if (x > 0.41421356237309504880) { add = 0.78539816339744830962; x = (x - 1.0) / (x + 1.0); }
  /* |x| <= tan(pi/8) = 0.414: the series x - x^3/3 + ... to x^27. */
  double z = x * x;
  double p = -1.0 / 27.0;
  p = fma(p, z, 1.0 / 25.0);
  p = fma(p, z, -1.0 / 23.0);
  p = fma(p, z, 1.0 / 21.0);
  p = fma(p, z, -1.0 / 19.0);
  p = fma(p, z, 1.0 / 17.0);
  p = fma(p, z, -1.0 / 15.0);
  p = fma(p, z, 1.0 / 13.0);
  p = fma(p, z, -1.0 / 11.0);
  p = fma(p, z, 1.0 / 9.0);
  p = fma(p, z, -1.0 / 7.0);
  p = fma(p, z, 1.0 / 5.0);
  p = fma(p, z, -1.0 / 3.0);
  return sign * (fma(p * z, x, x) + add);
}

__VCX_DEVICE_INLINE double __vcx_atan2(double y, double x) {
  if (x > 0.0) return __vcx_atan(y / x);
  if (x < 0.0) return __vcx_atan(y / x) + ((y < 0.0) ? -3.14159265358979323846 : 3.14159265358979323846);
  if (y > 0.0) return 1.5707963267948966192;
  if (y < 0.0) return -1.5707963267948966192;
  return 0.0;
}

__VCX_DEVICE_INLINE double __vcx_asin(double x) { return __vcx_atan2(x, sqrt(fma(-x, x, 1.0))); }
__VCX_DEVICE_INLINE double __vcx_acos(double x) { return __vcx_atan2(sqrt(fma(-x, x, 1.0)), x); }

#endif /* __VCX_DEVICELIB_MATH_H__ */
