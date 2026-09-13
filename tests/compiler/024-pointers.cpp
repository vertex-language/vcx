// §7.6.2.2 -- taking an address and reading through it, and the fact that a
// pointer parameter lets a function write into its caller's storage.
void set(int* p, int v) { *p = v; }
int get(int* p) { return *p; }
void swap(int* a, int* b) { int t = *a; *a = *b; *b = t; }

int main() {
    int x = 1;
    int y = 2;
    set(&x, 10);
    swap(&x, &y);

    int* p = &x;
    *p = *p + 5;

    return get(&x) * 100 + y + (p == &x);
}
