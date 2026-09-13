// 053-static-members.cpp
// Tests static member variables and static member functions:
// - static data member defined out of line
// - static member functions
// - static members accessed via class qualifier (Class::member)
// - static members accessed via instance (obj.member)
// - static constexpr members

struct Counter {
    static int count;
    static constexpr int Limit = 50;

    static int increment() {
        count++;
        return count;
    }

    static int add(int a, int b) {
        return a + b;
    }

    int get() const {
        return count;
    }
};

int Counter::count = 10;

int main() {
    // 1. Initial value: 10
    int c1 = Counter::count;

    // 2. Call static member function directly: 11
    int c2 = Counter::increment();

    // 3. Call static member function with arguments: 15
    int c3 = Counter::add(7, 8);

    // 4. Access via instance:
    Counter obj;
    int c4 = obj.count;       // 11
    int c5 = obj.increment(); // 12
    int c6 = obj.add(20, 22); // 42
    int c7 = obj.get();       // 12

    // 5. Static constexpr: 50
    int c8 = Counter::Limit;

    // Sum: 10 + 11 + 15 + 11 + 12 + 42 + 12 + 50 = 163
    return (c1 + c2 + c3 + c4 + c5 + c6 + c7 + c8) % 256;
}
