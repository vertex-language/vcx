// §8.5.3 -- the condition jumps to one label and runs from there to the end
// of the body, which is why fallthrough is the default and `break` is
// written at all.
int classify(int n) {
    switch (n) {
    case 0:
        return 100;
    case 1:
    case 2:
        return 200;
    case 3: {
        int doubled = n * 2;
        return 300 + doubled;
    }
    default:
        return 400;
    }
}

int falls_through(int n) {
    int total = 0;
    switch (n) {
    case 3: total = total + 3;
    case 2: total = total + 2;
    case 1: total = total + 1;
        break;
    case 0:
        total = total + 100;
    }
    return total;
}

int main() {
    int n = 0;
    n = n + classify(0) + classify(1) + classify(2) + classify(3) + classify(9);
    n = n + falls_through(3) + falls_through(1) * 2;
    return n % 251;
}
