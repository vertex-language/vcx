// printf from a kernel: the driver's vprintf, with the arguments packed
// as varargs are, ints and doubles and a string, one line per thread in
// thread order since one block runs them in lockstep here.
// expect: thread 0: 0 0.500000 hello 42
// expect: thread 1: 3 1.500000 hello 42
// expect: thread 2: 6 2.500000 hello 42
// expect: thread 3: 9 3.500000 hello 42
// expect: done
#include <cuda_runtime.h>
#include <stdio.h>

__global__ void say(long long big) {
  float f = threadIdx.x + 0.5f;
  printf("thread %d: %d %f %s %lld\n", threadIdx.x, threadIdx.x * 3, f, "hello", big);
}

int main() {
  say<<<1, 4>>>(42);
  cudaDeviceSynchronize();
  printf("done\n");
  return 0;
}
