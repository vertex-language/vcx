int global = 1;
static int internal = 2;
extern int elsewhere;

int f(int a, int b) {
    int local = a + b;
    const int frozen = local;
    return frozen;
}

void scopes() {
    int x = 1;
    {
        int x = 2;      // shadows, and does not redeclare
        (void)x;
    }
    (void)x;
}
