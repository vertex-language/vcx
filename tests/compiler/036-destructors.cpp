// Does a destructor run when its object's lifetime ends?
//
// §6.7.3/1 -- a local's lifetime ends at the closing brace, and §11.4.7
// says the destructor runs then, in the reverse order of construction. The
// side effect below is a count; the exit status says how many ran and in
// what order.

int trace;

struct Guard {
    int tag;
    Guard(int t) : tag(t) { trace = trace * 10 + t; }
    ~Guard() { trace = trace * 10 + tag + 4; }
};

int scoped() {
    Guard a(1);
    {
        Guard b(2);
    }
    Guard c(3);
    return trace;
}

int main() {
    int before = scoped();   // 1, 2, 6, 3 = 1263 -- then c and a go: 12637, 126375
    return trace - before * 100 - 75 == 0 ? 42 : 1;
}
