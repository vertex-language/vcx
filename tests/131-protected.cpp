// Protected members are reachable from a derived class, not from outside.
#include <cstdio>

class Account {
public:
    int balance() const { return balance_; }

protected:
    void credit(int n) { balance_ += n; }

private:
    int balance_ = 0;
};

class Savings : public Account {
public:
    void interest() { credit(balance() / 10 + 5); }
};

int main() {
    Savings s;
    s.interest();
    s.interest();
    std::printf("%d\n", s.balance());
    return 0;
}
