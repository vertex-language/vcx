// Do the two compilers agree on how a free function is called?
//
// Every integer width, a float and a double, a pointer, and a function
// with more arguments than there are registers, so the fifth and sixth go
// on the stack where the other side looks for them.

int add3(int a, int b, int c);
long long widen(int a, unsigned char b, short c, unsigned long long d);
double mix(int a, double b, float c, int d);
int six(int a, int b, int c, int d, int e, int f);
int through(int *p);

int main() {
    int x = 40;
    return add3(1, 2, 3) + (int)widen(1, 200, -3, 4) + (int)mix(1, 2.5, 0.5f, 4) + six(1, 2, 3, 4, 5, 6) + through(&x) - 40;
}
