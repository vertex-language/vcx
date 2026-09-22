// A struct reached through a pointer, with ->.
#include <cstdio>

struct Account {
    int id;
    long long balance;
};

void deposit(Account* a, long long amount) { a->balance += amount; }

int main() {
    Account acc{7, 100};
    deposit(&acc, 50);
    Account* p = &acc;
    p->id *= 3;
    std::printf("%d %lld\n", acc.id, p->balance);
    return 0;
}
