// §8.7.1 and §8.7.2 -- the two transfers out of a loop's body.
int main() {
    int total = 0;
    for (int i = 0; i < 100; ++i) {
        if (i == 7) break;
        total = total + 1;
    }

    for (int i = 0; i < 10; ++i) {
        if (i % 2 == 0) continue;
        total = total + 10;
    }

    int n = 0;
    while (1) {
        n = n + 1;
        if (n > 3) break;
    }
    return total + n;
}
