/*
 * __vcx_cuda_vector_types.h -- the CUDA vector types and dim3, laid out
 * as the toolkit's vector_types.h lays them out: the same names, the
 * same alignments, the same constructors. It claims the toolkit
 * header's guard so that the two are never both read.
 */
#ifndef __VECTOR_TYPES_H__
#define __VECTOR_TYPES_H__

#define __VCX_VEC1(name, T)                                                  \
  struct name {                                                              \
    T x;                                                                     \
  };
#define __VCX_VEC2(name, T, align)                                           \
  struct __attribute__((aligned(align))) name {                              \
    T x, y;                                                                  \
  };
#define __VCX_VEC3(name, T)                                                  \
  struct name {                                                              \
    T x, y, z;                                                               \
  };
#define __VCX_VEC4(name, T, align)                                           \
  struct __attribute__((aligned(align))) name {                              \
    T x, y, z, w;                                                            \
  };

__VCX_VEC1(char1, signed char)
__VCX_VEC1(uchar1, unsigned char)
__VCX_VEC2(char2, signed char, 2)
__VCX_VEC2(uchar2, unsigned char, 2)
__VCX_VEC3(char3, signed char)
__VCX_VEC3(uchar3, unsigned char)
__VCX_VEC4(char4, signed char, 4)
__VCX_VEC4(uchar4, unsigned char, 4)

__VCX_VEC1(short1, short)
__VCX_VEC1(ushort1, unsigned short)
__VCX_VEC2(short2, short, 4)
__VCX_VEC2(ushort2, unsigned short, 4)
__VCX_VEC3(short3, short)
__VCX_VEC3(ushort3, unsigned short)
__VCX_VEC4(short4, short, 8)
__VCX_VEC4(ushort4, unsigned short, 8)

__VCX_VEC1(int1, int)
__VCX_VEC1(uint1, unsigned int)
__VCX_VEC2(int2, int, 8)
__VCX_VEC2(uint2, unsigned int, 8)
__VCX_VEC3(int3, int)
__VCX_VEC3(uint3, unsigned int)
__VCX_VEC4(int4, int, 16)
__VCX_VEC4(uint4, unsigned int, 16)

__VCX_VEC1(long1, long)
__VCX_VEC1(ulong1, unsigned long)
__VCX_VEC2(long2, long, 2 * sizeof(long))
__VCX_VEC2(ulong2, unsigned long, 2 * sizeof(long))
__VCX_VEC3(long3, long)
__VCX_VEC3(ulong3, unsigned long)
__VCX_VEC4(long4, long, 16)
__VCX_VEC4(ulong4, unsigned long, 16)

__VCX_VEC1(longlong1, long long)
__VCX_VEC1(ulonglong1, unsigned long long)
__VCX_VEC2(longlong2, long long, 16)
__VCX_VEC2(ulonglong2, unsigned long long, 16)
__VCX_VEC3(longlong3, long long)
__VCX_VEC3(ulonglong3, unsigned long long)
__VCX_VEC4(longlong4, long long, 16)
__VCX_VEC4(ulonglong4, unsigned long long, 16)

__VCX_VEC1(float1, float)
__VCX_VEC2(float2, float, 8)
__VCX_VEC3(float3, float)
__VCX_VEC4(float4, float, 16)

__VCX_VEC1(double1, double)
__VCX_VEC2(double2, double, 16)
__VCX_VEC3(double3, double)
__VCX_VEC4(double4, double, 16)

#undef __VCX_VEC1
#undef __VCX_VEC2
#undef __VCX_VEC3
#undef __VCX_VEC4

struct dim3 {
  unsigned int x, y, z;
  __host__ __device__ constexpr dim3(unsigned int vx = 1, unsigned int vy = 1, unsigned int vz = 1)
      : x(vx), y(vy), z(vz) {}
  __host__ __device__ constexpr dim3(uint3 v) : x(v.x), y(v.y), z(v.z) {}
  __host__ __device__ constexpr operator uint3(void) const { return uint3{x, y, z}; }
};

typedef struct dim3 dim3;

#endif /* __VECTOR_TYPES_H__ */
