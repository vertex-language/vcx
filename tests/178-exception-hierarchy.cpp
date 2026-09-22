// A handler for a base class catches a derived exception; rethrow keeps the type.
#include <cstdio>
#include <stdexcept>

struct NotFound : std::runtime_error {
    NotFound() : std::runtime_error("not found") {}
};

void lookup() { throw NotFound(); }

void wrapper() {
    try {
        lookup();
    } catch (const std::exception&) {
        std::printf("logged\n");
        throw;
    }
}

int main() {
    try {
        wrapper();
    } catch (const NotFound& e) {
        std::printf("NotFound: %s\n", e.what());
    }
    try {
        throw std::out_of_range("index");
    } catch (const std::logic_error& e) {
        std::printf("logic: %s\n", e.what());
    }
    return 0;
}
