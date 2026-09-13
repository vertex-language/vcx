// §8.5.1 -- the branch, in both shapes.
int classify(int n) {
    if (n < 0) return 1;
    if (n == 0) return 2;
    if (n < 10) {
        return 3;
    } else {
        return 4;
    }
}

int main() {
    return classify(-5) + classify(0) * 3 + classify(5) * 9 + classify(50) * 27;
}
