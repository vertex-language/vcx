// A switch over many consecutive cases: the shape a jump table takes.
#include <cstdio>

int f(int v) {
    switch (v) {
    case 0: return 100;
    case 1: return 91;
    case 2: return 82;
    case 3: return 73;
    case 4: return 64;
    case 5: return 55;
    case 6: return 46;
    case 7: return 37;
    case 8: return 28;
    case 9: return 19;
    }
    return 0;
}

int main() {
    int sum = 0;
    for (int i = -2; i < 12; ++i) sum = sum * 3 + f(i);
    std::printf("%d\n", sum);
    return 0;
}
