// §8.6.1 -- the loop whose condition is read before the body.
int main() {
    int n = 0;
    int total = 0;
    while (n < 10) {
        total = total + n;
        n = n + 1;
    }

    int never = 0;
    while (never > 0) {
        total = total + 1000;
    }
    return total;
}
