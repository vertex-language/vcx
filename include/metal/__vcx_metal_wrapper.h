/*
 * __vcx_metal_wrapper.h -- read before the first line of every .metal
 * unit. It gives the unit what the Metal compiler gives one without an
 * #include: the address spaces, the kernel keyword, and the scalar and
 * vector type names MSL has built in.
 *
 * The address spaces are attributes underneath, and the analysis reads
 * them as MSL means them: on the object in `threadgroup float t[64]`, on
 * what the pointer points at in `device float* p` (see sema/metal.go).
 */
#ifndef __VCX_METAL_WRAPPER_H__
#define __VCX_METAL_WRAPPER_H__

/* ---- functions and address spaces ------------------------------------- */

#define kernel __attribute__((global))
#define vertex __attribute__((metal_vertex))
#define fragment __attribute__((metal_fragment))

#define device __attribute__((metal_device))
#define constant __attribute__((metal_constant))
#define threadgroup __attribute__((metal_threadgroup))
#define thread __attribute__((metal_thread))

/* ---- the scalar types ------------------------------------------------- */

typedef unsigned char uchar;
typedef unsigned short ushort;
typedef unsigned int uint;
typedef unsigned long ulong;
typedef __SIZE_TYPE__ size_t;
typedef __PTRDIFF_TYPE__ ptrdiff_t;

/* ---- the vector types ------------------------------------------------- */

/* What the built-in arguments are declared as. A plain struct for now:
 * members, and no arithmetic or swizzles, which MSL's vectors have and
 * vcx's do not yet. */
struct uint2 { uint x, y; };
struct uint3 { uint x, y, z; };
struct ushort2 { ushort x, y; };
struct ushort3 { ushort x, y, z; };

#endif /* __VCX_METAL_WRAPPER_H__ */
