// A function that calls itself, and two that call each other.
int fib(int n) {
    if (n < 2) return n;
    return fib(n - 1) + fib(n - 2);
}

int is_odd(int n);
int is_even(int n) { return n == 0 ? 1 : is_odd(n - 1); }
int is_odd(int n) { return n == 0 ? 0 : is_even(n - 1); }

int main() {
    return fib(10) + is_even(10) * 100 + is_odd(7) * 50;
}
