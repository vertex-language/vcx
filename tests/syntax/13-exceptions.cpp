// §14 [except] -- throw, the handler grammar, and the specifier that says a
// function has none.

struct E1 {}; struct E2 : E1 {};

// §14.2 [except.throw] -- throw with an operand, and the rethrow with none.
void thrower() {
    throw 1;
    throw E1();
    throw E1{};
    throw "literal";
    throw;
}

// §14.1 [except.pre] -- the try-block, and every exception-declaration form
// a handler can take.
void handlers() {
    try {
        thrower();
    } catch (int e) {
        (void)e;
    } catch (const E1& e) {
        (void)e;
    } catch (E1* e) {
        (void)e;
    } catch (E1) {
    } catch (const volatile E1&) {
    } catch (...) {
    }
}

// §14.1/2 -- a handler may rethrow, and a try-block nests.
void nested() {
    try {
        try {
            thrower();
        } catch (E2&) {
            throw;
        }
    } catch (E1&) {
    } catch (...) {
        throw;
    }
}

// §14.5 [except.spec] -- noexcept in its two spellings, and on a pointer.
void n1() noexcept;
void n2() noexcept(true);
void n3() noexcept(false);
void n4() noexcept(noexcept(n1()));
void (*pn)() noexcept = n1;
template <typename T> void n5() noexcept(sizeof(T) > 0);

// §14.4 [except.handle]/15 -- the function-try-block, on a plain function
// and on a constructor. The constructor form is the one the construct exists
// for: its handler covers the mem-initializer-list too, so a base or member
// constructor that throws is caught. The ctor-initializer sits *inside* the
// try, which is what §14.1's grammar says and what makes that possible.
int ftb() try {
    return 1;
} catch (...) {
    return 0;
}

struct Dtor {
    ~Dtor() try {
    } catch (...) {
    }
};

// §14.3 [except.ctor] -- a destructor runs on the way out, which is what
// makes the block below different from a goto; the grammar is the same.
struct Guard { Guard(); ~Guard(); };
void unwinds() {
    Guard g;
    try {
        Guard inner;
        thrower();
    } catch (...) {
    }
}

struct Ctor {
    int m;
    Ctor(int v) try : m(v) {
    } catch (...) {
    }
    Ctor() try {
    } catch (...) {
    }
};

struct Based : Guard {
    int m;
    Based(int v) try : Guard(), m(v) {
    } catch (...) {
    }
};
