// Does a free function get the name cl gives it?
//
// Every fundamental type in a parameter, an empty list, an ellipsis, a
// return type of each kind, and three overloads of one name that must come
// out as three symbols. `main` and an `extern "C"` function keep their
// spelling, which is the one thing about linkage §6.9.3.1 and §9.11 decide.

void f() {}
void f(int) {}
void f(int, int) {}
int g(int a) { return a; }
double h(float x, double y, long double z) { return x + y + z; }
void ints(signed char, unsigned char, char, short, unsigned short, unsigned int, long, unsigned long, long long, unsigned long long) {}
void wide(bool, wchar_t, char8_t, char16_t, char32_t) {}
void v(int, ...) {}
void only_dots(...) {}
void np(decltype(nullptr)) {}
long long big() { return 1; }
unsigned u() { return 1; }
extern "C" void c_linkage(int) {}
extern "C" {
    int another_c(void) { return 0; }
}
int main() { return 0; }
