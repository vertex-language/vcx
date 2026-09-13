// Do the two compilers agree on how a class travels through a call?
//
// A plain eight-byte struct goes in a register and comes back in one; a
// twenty-four-byte one goes by a copy in memory and comes back through
// the caller's storage; a class with a constructor of its own comes back
// through a hidden pointer whatever its size. Each is sent one way and
// received the other, so that both the caller's and the callee's half of
// each rule is checked in both directions.

struct Pair { int a, b; };
struct Wide { long long v[3]; };
struct Made {
    int n;
    Made(int k) : n(k) {}
};

int sum_pair(Pair p);
Pair make_pair(int a, int b);
long long sum_wide(Wide w);
Wide make_wide(long long a);
Made make_made(int k);
int read_made(Made m);

int main() {
    Pair p = make_pair(3, 4);
    Wide w = make_wide(5);
    Made m = make_made(6);
    return sum_pair(p) + (int)sum_wide(w) + read_made(m);   // 7 + 15 + 6 = 28
}
