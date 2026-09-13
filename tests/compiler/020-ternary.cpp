// §7.6.16 -- only one arm runs, which is observable when the other one
// would change something.
int bump(int* n) { *n = *n + 1; return *n; }

int main() {
    int calls = 0;
    int a = 1 ? 10 : bump(&calls);
    int b = 0 ? bump(&calls) : 20;
    int c = (a > b) ? a : b;
    int d = a < b ? 1 : a == b ? 2 : 3;
    return a + b + c + d + calls * 100;
}
