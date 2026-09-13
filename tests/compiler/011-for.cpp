// §8.6.3 -- the three-clause loop, and the increment that makes it one.
int main() {
    int total = 0;
    for (int i = 0; i < 10; ++i) {
        total = total + i;
    }
    for (int i = 10; i > 0; i--) {
        total = total + 1;
    }
    for (int i = 0, j = 10; i < j; ++i, --j) {
        total = total + 2;
    }
    return total;
}
