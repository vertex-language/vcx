// An aggregate with a base class, initialized with nested braces and designators.
#include <cstdio>

struct Named {
    const char* name;
};

struct Item : Named {
    int count;
    double price;
};

struct Order {
    Item item;
    int qty = 1;
};

int main() {
    Item a{{"pen"}, 3, 1.5};
    Order o{.item = {{"ink"}, 1, 4.0}, .qty = 2};
    Order d{a};
    std::printf("%s %d %g | %s %d | %s %d\n", a.name, a.count, a.price, o.item.name, o.qty, d.item.name, d.qty);
    return 0;
}
