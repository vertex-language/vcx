/*
 * __vcx_cuda_warp.h -- the warp-level primitives: shuffles, votes and
 * the bit-twiddling intrinsics, over the __nvvm_* builtins the device
 * pass lowers. A sub-warp width is folded into the lane arithmetic
 * here, so that the builtins move values between whole-warp lanes and
 * nothing else.
 */
#ifndef __VCX_CUDA_WARP_H__
#define __VCX_CUDA_WARP_H__

#if defined(__CUDA_ARCH__)

/* ---- bit casts --------------------------------------------------------- */

__VCX_DEVICE_INLINE int __float_as_int(float f) { return __nvvm_bitcast_f2i(f); }
__VCX_DEVICE_INLINE unsigned __float_as_uint(float f) { return (unsigned)__nvvm_bitcast_f2i(f); }
__VCX_DEVICE_INLINE float __int_as_float(int i) { return __nvvm_bitcast_i2f(i); }
__VCX_DEVICE_INLINE float __uint_as_float(unsigned i) { return __nvvm_bitcast_i2f((int)i); }
__VCX_DEVICE_INLINE long long __double_as_longlong(double d) { return __nvvm_bitcast_d2ll(d); }
__VCX_DEVICE_INLINE double __longlong_as_double(long long l) { return __nvvm_bitcast_ll2d(l); }

/* ---- bit twiddling ------------------------------------------------------ */

__VCX_DEVICE_INLINE int __popc(unsigned x) { return __builtin_popcount(x); }
__VCX_DEVICE_INLINE int __popcll(unsigned long long x) { return __builtin_popcountll(x); }
__VCX_DEVICE_INLINE int __clz(int x) { return x == 0 ? 32 : __builtin_clz((unsigned)x); }
__VCX_DEVICE_INLINE int __clzll(long long x) { return x == 0 ? 64 : __builtin_clzll((unsigned long long)x); }
__VCX_DEVICE_INLINE int __ffs(int x) { return x == 0 ? 0 : __builtin_ctz((unsigned)x) + 1; }
__VCX_DEVICE_INLINE int __ffsll(long long x) { return x == 0 ? 0 : __builtin_ctzll((unsigned long long)x) + 1; }
__VCX_DEVICE_INLINE unsigned __brev(unsigned x) { return __nvvm_brev32(x); }
__VCX_DEVICE_INLINE unsigned long long __brevll(unsigned long long x) { return __nvvm_brev64(x); }
__VCX_DEVICE_INLINE unsigned __byte_perm(unsigned a, unsigned b, unsigned s) { return __nvvm_prmt(a, b, s); }
__VCX_DEVICE_INLINE int __mul24(int a, int b) { return __nvvm_mul24_i(a, b); }
__VCX_DEVICE_INLINE unsigned __umul24(unsigned a, unsigned b) { return __nvvm_mul24_ui(a, b); }
__VCX_DEVICE_INLINE int __mulhi(int a, int b) { return __nvvm_mulhi_i(a, b); }
__VCX_DEVICE_INLINE unsigned __umulhi(unsigned a, unsigned b) { return __nvvm_mulhi_ui(a, b); }
__VCX_DEVICE_INLINE long long __mul64hi(long long a, long long b) { return __nvvm_mulhi_ll(a, b); }
__VCX_DEVICE_INLINE unsigned long long __umul64hi(unsigned long long a, unsigned long long b) { return __nvvm_mulhi_ull(a, b); }
__VCX_DEVICE_INLINE unsigned __funnelshift_l(unsigned lo, unsigned hi, unsigned s) { return __nvvm_fshl(hi, lo, s); }
__VCX_DEVICE_INLINE unsigned __funnelshift_r(unsigned lo, unsigned hi, unsigned s) { return __nvvm_fshr(hi, lo, s); }
__VCX_DEVICE_INLINE unsigned __sad(int a, int b, unsigned c) { return __nvvm_sad_i(a, b, c); }
__VCX_DEVICE_INLINE unsigned __usad(unsigned a, unsigned b, unsigned c) { return __nvvm_sad_ui(a, b, c); }

/* ---- shuffles ------------------------------------------------------------ */

/* The lane a width-w shuffle reads from: srcLane within this lane's
 * segment of w lanes for idx, this lane's index XOR laneMask for xor,
 * and the lane delta away for up and down, which read their own value
 * where the delta leaves the segment. */
__VCX_DEVICE_INLINE unsigned __vcx_shfl_idx_lane(int srcLane, int width) {
  unsigned lane = __nvvm_read_ptx_sreg_laneid();
  return (lane & ~(unsigned)(width - 1)) | ((unsigned)srcLane & (unsigned)(width - 1));
}
__VCX_DEVICE_INLINE unsigned __vcx_shfl_up_lane(unsigned delta, int width) {
  unsigned lane = __nvvm_read_ptx_sreg_laneid();
  unsigned base = lane & ~(unsigned)(width - 1);
  return lane - delta < base || delta > lane ? lane : lane - delta;
}
__VCX_DEVICE_INLINE unsigned __vcx_shfl_down_lane(unsigned delta, int width) {
  unsigned lane = __nvvm_read_ptx_sreg_laneid();
  unsigned top = (lane | (unsigned)(width - 1));
  return lane + delta > top ? lane : lane + delta;
}
__VCX_DEVICE_INLINE unsigned __vcx_shfl_xor_lane(int laneMask, int width) {
  unsigned lane = __nvvm_read_ptx_sreg_laneid();
  unsigned j = lane ^ (unsigned)laneMask;
  return j > (lane | (unsigned)(width - 1)) ? lane : j;
}

#define __VCX_SHFL_INT(T, conv_in, conv_out)                                                      \
  __VCX_DEVICE_INLINE T __shfl_sync(unsigned mask, T var, int srcLane, int width = warpSize) {   \
    return conv_out(__nvvm_shfl_sync_idx_i32(mask, conv_in(var), __vcx_shfl_idx_lane(srcLane, width), 0x1f)); \
  }                                                                                              \
  __VCX_DEVICE_INLINE T __shfl_up_sync(unsigned mask, T var, unsigned delta, int width = warpSize) { \
    return conv_out(__nvvm_shfl_sync_idx_i32(mask, conv_in(var), __vcx_shfl_up_lane(delta, width), 0x1f)); \
  }                                                                                              \
  __VCX_DEVICE_INLINE T __shfl_down_sync(unsigned mask, T var, unsigned delta, int width = warpSize) { \
    return conv_out(__nvvm_shfl_sync_idx_i32(mask, conv_in(var), __vcx_shfl_down_lane(delta, width), 0x1f)); \
  }                                                                                              \
  __VCX_DEVICE_INLINE T __shfl_xor_sync(unsigned mask, T var, int laneMask, int width = warpSize) { \
    return conv_out(__nvvm_shfl_sync_idx_i32(mask, conv_in(var), __vcx_shfl_xor_lane(laneMask, width), 0x1f)); \
  }

#define __VCX_AS_INT(x) ((int)(x))
#define __VCX_AS_IS(x) (x)
__VCX_SHFL_INT(int, __VCX_AS_IS, __VCX_AS_IS)
__VCX_SHFL_INT(unsigned, __VCX_AS_INT, (unsigned))
__VCX_SHFL_INT(float, __float_as_int, __int_as_float)
#undef __VCX_AS_INT
#undef __VCX_AS_IS
#undef __VCX_SHFL_INT

/* A 64-bit value moves as two halves. */
#define __VCX_SHFL_64(T, conv_in, conv_out, NAME, LANE, ARG)                                \
  __VCX_DEVICE_INLINE T NAME(unsigned mask, T var, ARG, int width = warpSize) {             \
    unsigned long long v = (unsigned long long)conv_in(var);                                \
    int lo = (int)(unsigned)v, hi = (int)(unsigned)(v >> 32);                               \
    unsigned j = __vcx_shfl##LANE##_lane(__vcx_arg, width);                                 \
    lo = __nvvm_shfl_sync_idx_i32(mask, lo, j, 0x1f);                                       \
    hi = __nvvm_shfl_sync_idx_i32(mask, hi, j, 0x1f);                                       \
    return conv_out((long long)(((unsigned long long)(unsigned)hi << 32) | (unsigned)lo)); \
  }
#define __VCX_SHFL_64_ALL(T, conv_in, conv_out)                                             \
  __VCX_SHFL_64(T, conv_in, conv_out, __shfl_sync, _idx, int __vcx_arg)                     \
  __VCX_SHFL_64(T, conv_in, conv_out, __shfl_up_sync, _up, unsigned __vcx_arg)              \
  __VCX_SHFL_64(T, conv_in, conv_out, __shfl_down_sync, _down, unsigned __vcx_arg)          \
  __VCX_SHFL_64(T, conv_in, conv_out, __shfl_xor_sync, _xor, int __vcx_arg)
#define __VCX_LL(x) ((long long)(x))
__VCX_SHFL_64_ALL(long long, __VCX_LL, __VCX_LL)
__VCX_SHFL_64_ALL(unsigned long long, __VCX_LL, (unsigned long long))
__VCX_SHFL_64_ALL(double, __double_as_longlong, __longlong_as_double)
#undef __VCX_LL
#undef __VCX_SHFL_64_ALL
#undef __VCX_SHFL_64

#if __SIZEOF_LONG__ == 8
__VCX_DEVICE_INLINE long __shfl_sync(unsigned mask, long var, int srcLane, int width = warpSize) { return (long)__shfl_sync(mask, (long long)var, srcLane, width); }
__VCX_DEVICE_INLINE unsigned long __shfl_sync(unsigned mask, unsigned long var, int srcLane, int width = warpSize) { return (unsigned long)__shfl_sync(mask, (unsigned long long)var, srcLane, width); }
__VCX_DEVICE_INLINE long __shfl_up_sync(unsigned mask, long var, unsigned delta, int width = warpSize) { return (long)__shfl_up_sync(mask, (long long)var, delta, width); }
__VCX_DEVICE_INLINE unsigned long __shfl_up_sync(unsigned mask, unsigned long var, unsigned delta, int width = warpSize) { return (unsigned long)__shfl_up_sync(mask, (unsigned long long)var, delta, width); }
__VCX_DEVICE_INLINE long __shfl_down_sync(unsigned mask, long var, unsigned delta, int width = warpSize) { return (long)__shfl_down_sync(mask, (long long)var, delta, width); }
__VCX_DEVICE_INLINE unsigned long __shfl_down_sync(unsigned mask, unsigned long var, unsigned delta, int width = warpSize) { return (unsigned long)__shfl_down_sync(mask, (unsigned long long)var, delta, width); }
__VCX_DEVICE_INLINE long __shfl_xor_sync(unsigned mask, long var, int laneMask, int width = warpSize) { return (long)__shfl_xor_sync(mask, (long long)var, laneMask, width); }
__VCX_DEVICE_INLINE unsigned long __shfl_xor_sync(unsigned mask, unsigned long var, int laneMask, int width = warpSize) { return (unsigned long)__shfl_xor_sync(mask, (unsigned long long)var, laneMask, width); }
#else
__VCX_DEVICE_INLINE long __shfl_sync(unsigned mask, long var, int srcLane, int width = warpSize) { return (long)__shfl_sync(mask, (int)var, srcLane, width); }
__VCX_DEVICE_INLINE unsigned long __shfl_sync(unsigned mask, unsigned long var, int srcLane, int width = warpSize) { return (unsigned long)__shfl_sync(mask, (unsigned)var, srcLane, width); }
__VCX_DEVICE_INLINE long __shfl_up_sync(unsigned mask, long var, unsigned delta, int width = warpSize) { return (long)__shfl_up_sync(mask, (int)var, delta, width); }
__VCX_DEVICE_INLINE unsigned long __shfl_up_sync(unsigned mask, unsigned long var, unsigned delta, int width = warpSize) { return (unsigned long)__shfl_up_sync(mask, (unsigned)var, delta, width); }
__VCX_DEVICE_INLINE long __shfl_down_sync(unsigned mask, long var, unsigned delta, int width = warpSize) { return (long)__shfl_down_sync(mask, (int)var, delta, width); }
__VCX_DEVICE_INLINE unsigned long __shfl_down_sync(unsigned mask, unsigned long var, unsigned delta, int width = warpSize) { return (unsigned long)__shfl_down_sync(mask, (unsigned)var, delta, width); }
__VCX_DEVICE_INLINE long __shfl_xor_sync(unsigned mask, long var, int laneMask, int width = warpSize) { return (long)__shfl_xor_sync(mask, (int)var, laneMask, width); }
__VCX_DEVICE_INLINE unsigned long __shfl_xor_sync(unsigned mask, unsigned long var, int laneMask, int width = warpSize) { return (unsigned long)__shfl_xor_sync(mask, (unsigned)var, laneMask, width); }
#endif

/* ---- votes --------------------------------------------------------------- */

__VCX_DEVICE_INLINE int __all_sync(unsigned mask, int pred) { return __nvvm_vote_all_sync(mask, pred); }
__VCX_DEVICE_INLINE int __any_sync(unsigned mask, int pred) { return __nvvm_vote_any_sync(mask, pred); }
__VCX_DEVICE_INLINE unsigned __ballot_sync(unsigned mask, int pred) { return __nvvm_vote_ballot_sync(mask, pred); }
__VCX_DEVICE_INLINE unsigned __activemask(void) { return __nvvm_vote_ballot_sync(0xffffffffu, 1); }

#endif /* __CUDA_ARCH__ */
#endif /* __VCX_CUDA_WARP_H__ */
