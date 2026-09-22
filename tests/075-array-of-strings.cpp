// An array of string pointers, walked and indexed.
#include <cstdio>

int main() {
    const char* days[] = {"mon", "tue", "wed", "thu", "fri"};
    int total = 0;
    for (const char* d : days)
        for (const char* c = d; *c; ++c) total += *c;
    std::printf("%s %d\n", days[2], total);
    return 0;
}
