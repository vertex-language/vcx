// 056-namespaces.cpp
// Tests namespaces in C++:
// - nested namespaces (namespace A::B)
// - reopened namespaces
// - namespace alias (namespace N = A::B)
// - using namespace directive at file and block scope
// - using declaration (using A::B::func)
// - unnamed / anonymous namespace

namespace Math::Arithmetic {
    int add(int a, int b) {
        return a + b;
    }

    int sub(int a, int b) {
        return a - b;
    }
}

namespace Math::Arithmetic {
    int mul(int a, int b) {
        return a * b;
    }
}

namespace MA = Math::Arithmetic;

namespace Utility {
    int base_val = 100;
}

namespace {
    int secret = 77;
}

int test_nested_and_alias() {
    int a = Math::Arithmetic::add(10, 20); // 30
    int b = MA::sub(50, 15);               // 35
    int c = MA::mul(3, 4);                 // 12
    return a + b + c;                      // 77
}

int test_using_directive() {
    using namespace Utility;
    return base_val + 50; // 150
}

int test_using_decl() {
    using Math::Arithmetic::mul;
    return mul(10, 5); // 50
}

int test_anon_namespace() {
    return secret + 23; // 100
}

int main() {
    int r1 = test_nested_and_alias();   // 77
    int r2 = test_using_directive();    // 150
    int r3 = test_using_decl();         // 50
    int r4 = test_anon_namespace();     // 100

    // Total: 77 + 150 + 50 + 100 = 377
    // 377 % 256 = 121
    return (r1 + r2 + r3 + r4) % 256;
}
