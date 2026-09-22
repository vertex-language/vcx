// A two-dimensional array and a matrix product.
#include <cstdio>

int main() {
    int a[2][3] = {{1, 2, 3}, {4, 5, 6}};
    int b[3][2] = {{7, 8}, {9, 10}, {11, 12}};
    int c[2][2] = {};
    for (int i = 0; i < 2; ++i)
        for (int j = 0; j < 2; ++j)
            for (int k = 0; k < 3; ++k) c[i][j] += a[i][k] * b[k][j];
    std::printf("%d %d %d %d\n", c[0][0], c[0][1], c[1][0], c[1][1]);
    return 0;
}
