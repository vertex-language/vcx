// A kernel template and a __host__ __device__ function: the instance is
// the kernel, and the function is lowered on the device.
// grid: 1
// block: 4
// expect: 0 10 20 30
template <class T> __host__ __device__ T scale(T v, T k) { return v * k; }

template <class T> __global__ void kernel(T *out, T k) {
  out[threadIdx.x] = scale<T>((T)threadIdx.x, k);
}

__global__ void test(int *out) {
  // A kernel cannot launch another here; the instance is reached by name
  // so that it is emitted, and this body does its work directly.
  out[threadIdx.x] = scale<int>((int)threadIdx.x, 10);
}
