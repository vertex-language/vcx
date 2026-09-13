// §8.6.2 -- the body runs before the condition is read, which is the whole
// difference from a while and is observable when the condition is false.
int main() {
    int n = 0;
    int runs = 0;
    do {
        runs = runs + 1;
        n = n + 5;
    } while (n < 20);

    int never = 100;
    int alsoRuns = 0;
    do {
        alsoRuns = alsoRuns + 1;
    } while (never < 0);

    return runs * 10 + alsoRuns + n;
}
