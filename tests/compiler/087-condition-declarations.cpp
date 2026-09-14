// Is a declaration in an if's condition a variable the arms can use?
//
// §8.5.1 [stmt.select.general] -- `if (T x = e)` declares x, tests it
// converted to bool, and keeps it in scope through both arms and no
// further. The name used to go undeclared, so the arm that read it did
// not compile.

int* find(int* values, int n, int want) {
    for (int i = 0; i < n; i++)
        if (values[i] == want)
            return &values[i];
    return nullptr;
}

int main() {
    int values[4] = {3, 5, 7, 9};
    int total = 0;
    if (int* p = find(values, 4, 7))
        total += *p;
    if (int* p = find(values, 4, 8))
        total += 100;
    else
        total += p == nullptr ? 1 : 1000;
    if (int n = total - 8)
        total += n;
    else
        total += 50;
    return total;   // 7, then 1, then 8 - 8 = 0 takes the else: 58
}
