// §9.12 [dcl.attr] -- the attribute grammar, which allows an attribute in
// far more places than any of them are meaningful.

// §9.12.1 [dcl.attr.grammar]: the shapes.
[[]] int empty_list;
[[nodiscard]] int a1();
[[nodiscard, deprecated]] int a2();
[[nodiscard]] [[deprecated]] int a3();
[[deprecated("use a3")]] int a4();
[[using vendor: attr1, attr2]] int a5();      // attribute-using-prefix
[[vendor::unknown]] int a6();                 // unknown -- ignored, §9.12.1/6
[[vendor::unknown(1, "two", 3.0)]] int a7();  // ...with a balanced token-seq

// §9.12.3 through §9.12.12 -- the standard attributes, each where it goes.
[[noreturn]] void terminates();
[[nodiscard("the value matters")]] int checked();
struct [[nodiscard]] Handle {};
enum class [[deprecated]] Old { A };

void locals() {
    [[maybe_unused]] int unused = 0;
    int x = 0;
    switch (x) {
    case 0:
        x = 1;
        [[fallthrough]];
    case 1:
        break;
    }
    if (x) [[likely]] { x = 2; } else [[unlikely]] { x = 3; }
    while (x) [[likely]] { break; }
    [[assume(x >= 0)]];                       // C++23, §9.12.2
}

struct Layout {
    [[no_unique_address]] struct {} empty;
    int m;
};

// §9.12.1/4 -- an attribute may sit on almost any declarator position: the
// declaration, the declarator, the parameter, the type, the base-specifier.
[[deprecated]] int decl_attr;
int* [[vendor::ptr]] on_pointer;
int on_param([[maybe_unused]] int p);
struct Base {};
struct Derived : [[vendor::base]] Base {};
enum E { [[deprecated]] Enumerator, [[maybe_unused]] Another = 4 };
namespace [[deprecated]] DeprecatedNS {}

// An alignment-specifier is an attribute by grammar, §9.12.2.
alignas(16) int aligned;
alignas(double) int aligned_as;
struct alignas(32) Aligned { int m; };
void aligned_param(alignas(8) int p);
