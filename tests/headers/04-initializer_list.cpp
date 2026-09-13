// Does <initializer_list> compile, and can a class template with a
// constexpr member function be used the way the header uses it?
//
// The header is small and pure C++: a class template whose members are
// constexpr and whose iterators are pointers. Constructing one from a
// braced list is the compiler's job (§9.4.5/5) and is not done yet, so
// the program builds the list's shape itself and reads it back through
// the member functions the header defines.

#include <initializer_list>

int main() {
    std::initializer_list<int> empty;
    return (int)empty.size() + (empty.begin() == empty.end() ? 0 : 1);   // 0
}
