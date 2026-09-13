// §7.6.1.2 -- a subscript is pointer arithmetic, and §7.3.3 makes an array
// decay to a pointer to its first element the moment it is used as a value.
int sum(int* a, int n) {
    int total = 0;
    for (int i = 0; i < n; ++i) total = total + a[i];
    return total;
}

int main() {
    int a[5] = {1, 2, 3, 4, 5};
    a[0] = 10;
    a[4] = a[4] + 5;

    int total = sum(a, 5);
    int second = *(a + 1);
    return total + second;
}
