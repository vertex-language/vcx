// Does `x = {a, b}` assign what `x = T{a, b}` would?
//
// §7.6.19 [expr.assign]/9 -- a braced-init-list on the right of an
// assignment to a class builds a temporary of the class from it and
// assigns that; to a scalar it is the one value in the braces. The list
// used to be typed as an array of its first element's type, and the
// assignment refused.

struct Point { int x; int y; };
struct Tagged { long long id; Point where; };

int main() {
    Point p{1, 2};
    p = {3, 4};
    Tagged t{};
    t = {9, {5, 6}};
    Tagged elided{};
    elided = {7, 8, 9};
    int n = 0;
    n = {42};
    Point zero{5, 5};
    zero = {};
    return p.x + p.y + t.id + t.where.x + t.where.y     // 27
         + elided.where.y - elided.id + n              // 2 + 42
         + zero.x + zero.y;                            // 0 -> 71
}
