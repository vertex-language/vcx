// §8.7.4 [stmt.return]/2 -- the returned expression must convert to the
// declared return type.
struct S { int m; };
int f() {
    S s{1};
    return s;
}
