/*
 * __vcx_cuda_atomics.h -- the CUDA atomic functions over the __nvvm_atom_*
 * builtins, as clang's __clang_cuda_device_functions.h defines them. A
 * plain atomic is device scope and relaxed, as PTX's atom is; the _block
 * and _system forms narrow and widen the scope.
 *
 * Only what the hardware does in one instruction is a builtin: the
 * float and double exchanges move through their bits, and atomicInc
 * and atomicDec are compare-and-swap loops, which is what nvcc emits for
 * them on every SM before their own instruction existed.
 */
#ifndef __VCX_CUDA_ATOMICS_H__
#define __VCX_CUDA_ATOMICS_H__

#if defined(__CUDA_ARCH__)

#define __VCX_ATOMIC_SCOPES(DEF)                                                         \
  DEF(, __nvvm_atom_)                                                                    \
  DEF(_block, __nvvm_atom_cta_)                                                          \
  DEF(_system, __nvvm_atom_sys_)

#define __VCX_ATOMIC_RMW(SUFFIX, B)                                                      \
  __VCX_DEVICE_INLINE int atomicAdd##SUFFIX(int *p, int v) { return B##add_gen_i(p, v); }   \
  __VCX_DEVICE_INLINE unsigned atomicAdd##SUFFIX(unsigned *p, unsigned v) { return (unsigned)B##add_gen_i((int *)p, (int)v); } \
  __VCX_DEVICE_INLINE unsigned long long atomicAdd##SUFFIX(unsigned long long *p, unsigned long long v) { return (unsigned long long)B##add_gen_ll((long long *)p, (long long)v); } \
  __VCX_DEVICE_INLINE float atomicAdd##SUFFIX(float *p, float v) { return B##add_gen_f(p, v); } \
  __VCX_DEVICE_INLINE double atomicAdd##SUFFIX(double *p, double v) { return B##add_gen_d(p, v); } \
  __VCX_DEVICE_INLINE int atomicSub##SUFFIX(int *p, int v) { return B##add_gen_i(p, -v); }  \
  __VCX_DEVICE_INLINE unsigned atomicSub##SUFFIX(unsigned *p, unsigned v) { return (unsigned)B##add_gen_i((int *)p, -(int)v); } \
  __VCX_DEVICE_INLINE int atomicExch##SUFFIX(int *p, int v) { return B##xchg_gen_i(p, v); } \
  __VCX_DEVICE_INLINE unsigned atomicExch##SUFFIX(unsigned *p, unsigned v) { return (unsigned)B##xchg_gen_i((int *)p, (int)v); } \
  __VCX_DEVICE_INLINE unsigned long long atomicExch##SUFFIX(unsigned long long *p, unsigned long long v) { return (unsigned long long)B##xchg_gen_ll((long long *)p, (long long)v); } \
  __VCX_DEVICE_INLINE float atomicExch##SUFFIX(float *p, float v) { return __int_as_float(B##xchg_gen_i((int *)p, __float_as_int(v))); } \
  __VCX_DEVICE_INLINE int atomicMin##SUFFIX(int *p, int v) { return B##min_gen_i(p, v); }   \
  __VCX_DEVICE_INLINE unsigned atomicMin##SUFFIX(unsigned *p, unsigned v) { return B##min_gen_ui(p, v); } \
  __VCX_DEVICE_INLINE long long atomicMin##SUFFIX(long long *p, long long v) { return B##min_gen_ll(p, v); } \
  __VCX_DEVICE_INLINE unsigned long long atomicMin##SUFFIX(unsigned long long *p, unsigned long long v) { return B##min_gen_ull(p, v); } \
  __VCX_DEVICE_INLINE int atomicMax##SUFFIX(int *p, int v) { return B##max_gen_i(p, v); }   \
  __VCX_DEVICE_INLINE unsigned atomicMax##SUFFIX(unsigned *p, unsigned v) { return B##max_gen_ui(p, v); } \
  __VCX_DEVICE_INLINE long long atomicMax##SUFFIX(long long *p, long long v) { return B##max_gen_ll(p, v); } \
  __VCX_DEVICE_INLINE unsigned long long atomicMax##SUFFIX(unsigned long long *p, unsigned long long v) { return B##max_gen_ull(p, v); } \
  __VCX_DEVICE_INLINE int atomicAnd##SUFFIX(int *p, int v) { return B##and_gen_i(p, v); }   \
  __VCX_DEVICE_INLINE unsigned atomicAnd##SUFFIX(unsigned *p, unsigned v) { return (unsigned)B##and_gen_i((int *)p, (int)v); } \
  __VCX_DEVICE_INLINE unsigned long long atomicAnd##SUFFIX(unsigned long long *p, unsigned long long v) { return (unsigned long long)B##and_gen_ll((long long *)p, (long long)v); } \
  __VCX_DEVICE_INLINE int atomicOr##SUFFIX(int *p, int v) { return B##or_gen_i(p, v); }     \
  __VCX_DEVICE_INLINE unsigned atomicOr##SUFFIX(unsigned *p, unsigned v) { return (unsigned)B##or_gen_i((int *)p, (int)v); } \
  __VCX_DEVICE_INLINE unsigned long long atomicOr##SUFFIX(unsigned long long *p, unsigned long long v) { return (unsigned long long)B##or_gen_ll((long long *)p, (long long)v); } \
  __VCX_DEVICE_INLINE int atomicXor##SUFFIX(int *p, int v) { return B##xor_gen_i(p, v); }   \
  __VCX_DEVICE_INLINE unsigned atomicXor##SUFFIX(unsigned *p, unsigned v) { return (unsigned)B##xor_gen_i((int *)p, (int)v); } \
  __VCX_DEVICE_INLINE unsigned long long atomicXor##SUFFIX(unsigned long long *p, unsigned long long v) { return (unsigned long long)B##xor_gen_ll((long long *)p, (long long)v); } \
  __VCX_DEVICE_INLINE int atomicCAS##SUFFIX(int *p, int cmp, int v) { return B##cas_gen_i(p, cmp, v); } \
  __VCX_DEVICE_INLINE unsigned atomicCAS##SUFFIX(unsigned *p, unsigned cmp, unsigned v) { return (unsigned)B##cas_gen_i((int *)p, (int)cmp, (int)v); } \
  __VCX_DEVICE_INLINE unsigned long long atomicCAS##SUFFIX(unsigned long long *p, unsigned long long cmp, unsigned long long v) { return (unsigned long long)B##cas_gen_ll((long long *)p, (long long)cmp, (long long)v); } \
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

__VCX_ATOMIC_SCOPES(__VCX_ATOMIC_RMW)

#undef __VCX_ATOMIC_RMW
#undef __VCX_ATOMIC_SCOPES

#endif /* __CUDA_ARCH__ */
#endif /* __VCX_CUDA_ATOMICS_H__ */
