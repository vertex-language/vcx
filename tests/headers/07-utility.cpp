// Does <utility> compile, and do pair, swap, move and exchange work?
//
// The header is where the library's smaller machines live: pair with its
// member templates, swap through move, exchange, integer_sequence,
// forward. The program uses the ones that need no exceptions and no
// operator<=>, and adds their results up.

#include <utility>

int main() {
    int a = 3, b = 4;
    std::swap(a, b);                 // a = 4, b = 3
    int old = std::exchange(a, 10);  // old = 4, a = 10
    std::pair<int, int> p(a, b);     // 10, 3
    return p.first + p.second + old + b;  // 10 + 3 + 4 + 3 = 20
}
