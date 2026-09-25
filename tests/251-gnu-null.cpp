// GCC's __null, which <stddef.h> makes NULL in C++: an integer null
// pointer constant of type long. It converts to any pointer, initializes
// an integer, and is a long's size.
#include <stddef.h>
#include <stdio.h>
int main() {
    void* p = NULL;
    int* q = NULL;
    long n = NULL;
    printf("%d %d %ld %zu %d\n", p == NULL, q == 0, n, sizeof(NULL), (int)sizeof(__null));
    return 0;
}
