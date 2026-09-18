/*
 * __vcx_hip_atomics.h -- the HIP atomic functions over the __hip_atomic_*
 * builtins, as ROCm's amd_hip_atomic.h defines them: relaxed, at agent
 * scope unless the _system form widens it.
 */
#ifndef __VCX_HIP_ATOMICS_H__
#define __VCX_HIP_ATOMICS_H__


#define __VCX_HIP_ATOMIC_SCOPES(DEF)                                                     \
  DEF(, __HIP_MEMORY_SCOPE_AGENT)                                                        \
  DEF(_system, __HIP_MEMORY_SCOPE_SYSTEM)

#define __VCX_HIP_RMW_T(SUFFIX, SCOPE, T)                                                \
  __VCX_DEVICE_INLINE T atomicAdd##SUFFIX(T *p, T v) { return __hip_atomic_fetch_add(p, v, __ATOMIC_RELAXED, SCOPE); } \
  __VCX_DEVICE_INLINE T atomicSub##SUFFIX(T *p, T v) { return __hip_atomic_fetch_sub(p, v, __ATOMIC_RELAXED, SCOPE); } \
  __VCX_DEVICE_INLINE T atomicExch##SUFFIX(T *p, T v) { return __hip_atomic_exchange(p, v, __ATOMIC_RELAXED, SCOPE); } \
  __VCX_DEVICE_INLINE T atomicMin##SUFFIX(T *p, T v) { return __hip_atomic_fetch_min(p, v, __ATOMIC_RELAXED, SCOPE); } \
  __VCX_DEVICE_INLINE T atomicMax##SUFFIX(T *p, T v) { return __hip_atomic_fetch_max(p, v, __ATOMIC_RELAXED, SCOPE); } \
  __VCX_DEVICE_INLINE T atomicAnd##SUFFIX(T *p, T v) { return __hip_atomic_fetch_and(p, v, __ATOMIC_RELAXED, SCOPE); } \
  __VCX_DEVICE_INLINE T atomicOr##SUFFIX(T *p, T v) { return __hip_atomic_fetch_or(p, v, __ATOMIC_RELAXED, SCOPE); } \
  __VCX_DEVICE_INLINE T atomicXor##SUFFIX(T *p, T v) { return __hip_atomic_fetch_xor(p, v, __ATOMIC_RELAXED, SCOPE); } \
  __VCX_DEVICE_INLINE T atomicCAS##SUFFIX(T *p, T cmp, T v) {                            \
    __hip_atomic_compare_exchange_strong(p, &cmp, v, __ATOMIC_RELAXED, __ATOMIC_RELAXED, SCOPE); \
    return cmp;                                                                          \
  }

#define __VCX_HIP_RMW(SUFFIX, SCOPE)                                                     \
  __VCX_HIP_RMW_T(SUFFIX, SCOPE, int)                                                    \
  __VCX_HIP_RMW_T(SUFFIX, SCOPE, unsigned int)                                           \
  __VCX_HIP_RMW_T(SUFFIX, SCOPE, long long)                                              \
  __VCX_HIP_RMW_T(SUFFIX, SCOPE, unsigned long long)                                     \
  __VCX_DEVICE_INLINE float atomicAdd##SUFFIX(float *p, float v) { return __hip_atomic_fetch_add(p, v, __ATOMIC_RELAXED, SCOPE); } \
  __VCX_DEVICE_INLINE double atomicAdd##SUFFIX(double *p, double v) { return __hip_atomic_fetch_add(p, v, __ATOMIC_RELAXED, SCOPE); } \
  __VCX_DEVICE_INLINE float atomicExch##SUFFIX(float *p, float v) { return __hip_atomic_exchange(p, v, __ATOMIC_RELAXED, SCOPE); } \
  __VCX_DEVICE_INLINE unsigned atomicInc##SUFFIX(unsigned *p, unsigned v) {              \
    unsigned old = *(volatile unsigned *)p, seen;                                        \
    do {                                                                                 \
      seen = old;                                                                        \
      old = atomicCAS##SUFFIX(p, seen, seen >= v ? 0u : seen + 1u);                      \
    } while (old != seen);                                                               \
    return old;                                                                          \
  }                                                                                      \
  __VCX_DEVICE_INLINE unsigned atomicDec##SUFFIX(unsigned *p, unsigned v) {              \
    unsigned old = *(volatile unsigned *)p, seen;                                        \
    do {                                                                                 \
      seen = old;                                                                        \
      old = atomicCAS##SUFFIX(p, seen, seen == 0u || seen > v ? v : seen - 1u);          \
    } while (old != seen);                                                               \
    return old;                                                                          \
  }

__VCX_HIP_ATOMIC_SCOPES(__VCX_HIP_RMW)

#undef __VCX_HIP_RMW
#undef __VCX_HIP_RMW_T
#undef __VCX_HIP_ATOMIC_SCOPES

#endif /* __VCX_HIP_ATOMICS_H__ */
