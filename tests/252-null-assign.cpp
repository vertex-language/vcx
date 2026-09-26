// A null pointer constant assigned to a pointer: NULL is GCC's __null,
// an integer of type long that converts to any pointer.
#include <stddef.h>
#include <stdio.h>
struct Op;
struct D { Op** h; void* p; int t; };
int main() {
    D d = {(Op**)&d, &d, 1};
    d.h = NULL;
    d.p = 0;
    d.t = 0;
    printf("%d %d %d\n", d.h == NULL, d.p == nullptr, d.t);
    return 0;
}
