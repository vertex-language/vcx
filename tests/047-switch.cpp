// switch with cases, a default, and fallthrough.
#include <cstdio>
#include <initializer_list>

int score(int v) {
    int s = 0;
    switch (v) {
    case 1:
        s += 1;
        [[fallthrough]];
    case 2:
        s += 10;
        break;
    case 7:
    case 8:
        s = 78;
        break;
    default:
        s = -1;
    }
    return s;
}

int main() {
    for (int v : {1, 2, 3, 7, 8}) std::printf("%d ", score(v));
    std::printf("\n");
    return 0;
}
