// Calls, and §6.3's rule that a *declaration* is what a call needs: a
// function may be called before its definition so long as it has been
// declared, which is why the prototypes at the top are there and why C++
// does not have C's implicit declaration to fall back on.
int square(int n);
int cube(int n);
int twice(int n);

int main() {
    return square(5) + cube(3) - twice(4);
}

int square(int n) { return n * n; }
int cube(int n) { return n * square(n); }
int twice(int n) { return n + n; }
