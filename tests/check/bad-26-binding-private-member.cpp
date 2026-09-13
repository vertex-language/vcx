// §9.6/5 -- all non-static data members must be public in a structured binding decomposition.
class Secret {
    int x;
    int y;
public:
    Secret(int a, int b) : x(a), y(b) {}
};

int f() {
    Secret s{1, 2};
    auto [a, b] = s;
    return a;
}
