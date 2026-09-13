// Does a global with a constructor get constructed before main?
//
// §6.9.3.3 -- an object whose initializer is not a constant is initialized
// by code that runs before main, in declaration order within the unit.
// Nothing in the data section can hold a constructed object, so a global
// `Counter c(5)` is a call the program never wrote, made from a function
// the program never named, found through a pointer in a section the CRT
// walks on its way to main. Three of them here, each reading the one
// before, so that the order is observable and not only the fact.

int calls;

struct Counter {
    int value;
    Counter(int start) : value(start + calls++) {}
};

int base() { return 10; }

Counter first(base());        // 10 + 0
Counter second(first.value);  // 10 + 1
Counter third(second.value);  // 11 + 2
int plain = base() * 2;       // 20, not a constant either

int main() {
    return first.value + second.value + third.value + plain + calls;   // 10 + 11 + 13 + 20 + 3 = 57
}
