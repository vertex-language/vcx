// A destructor asks whether it runs because of unwinding.
#include <cstdio>
#include <exception>

struct Transaction {
    int start = std::uncaught_exceptions();
    ~Transaction() { std::printf(std::uncaught_exceptions() > start ? "rollback\n" : "commit\n"); }
};

int main() {
    {
        Transaction t;
    }
    try {
        Transaction t;
        throw 1;
    } catch (int) {
        std::printf("caught\n");
    }
    return 0;
}
