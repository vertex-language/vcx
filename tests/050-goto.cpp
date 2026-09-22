// goto, forward and backward.
#include <cstdio>

int main() {
    int i = 0, sum = 0;
again:
    sum += i;
    if (++i < 10) goto again;
    if (sum > 40) goto done;
    sum = -1;
done:
    std::printf("%d\n", sum);
    return 0;
}
