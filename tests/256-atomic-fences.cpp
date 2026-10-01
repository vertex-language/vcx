// GCC's fence builtins and the memory orders they take: each lowers to a
// fence (none for relaxed), and code around them runs as written.
#include <cstdio>

static int data, flag;

int main() {
    __atomic_thread_fence(__ATOMIC_SEQ_CST);
    __atomic_thread_fence(__ATOMIC_ACQUIRE);
    __atomic_thread_fence(__ATOMIC_RELEASE);
    __atomic_thread_fence(__ATOMIC_ACQ_REL);
    __atomic_thread_fence(__ATOMIC_CONSUME);
    __atomic_thread_fence(__ATOMIC_RELAXED);
    __atomic_signal_fence(__ATOMIC_SEQ_CST);
    __sync_synchronize();

    // Publish, then read back behind the matching fence.
    data = 42;
    __atomic_thread_fence(__ATOMIC_RELEASE);
    flag = 1;
    if (flag) {
        __atomic_thread_fence(__ATOMIC_ACQUIRE);
        printf("%d\n", data);
    }

    printf("%d %d %d %d %d %d\n", __ATOMIC_RELAXED, __ATOMIC_CONSUME, __ATOMIC_ACQUIRE,
           __ATOMIC_RELEASE, __ATOMIC_ACQ_REL, __ATOMIC_SEQ_CST);
#if __has_builtin(__atomic_thread_fence) && __has_builtin(__sync_synchronize)
    printf("fences are builtins\n");
#endif
    return 0;
}
