// §9.3.4.7 [dcl.fct.default]
// Default arguments for free functions, member functions, constructors, and forward declarations.

int compute(int x, int y = 10, int z = 20) {
    return x + y + z;
}

void forward_decl(int a, int b = 5);
void forward_decl(int a, int b) {}

struct Item {
    int val;
    Item(int v = 42) : val(v) {}
    int get_val(int bonus = 0) { return val + bonus; }
};

void check_defaults() {
    int a = compute(1);
    int b = compute(1, 2);
    int c = compute(1, 2, 3);
    forward_decl(10);
    forward_decl(10, 20);

    Item it_def;
    Item it_val(100);
    int v1 = it_def.get_val();
    int v2 = it_def.get_val(5);
}
