// exception_ptr carries an exception out of a handler; nested exceptions chain.
#include <cstdio>
#include <exception>
#include <stdexcept>

std::exception_ptr capture() {
    try {
        throw std::runtime_error("stored");
    } catch (...) {
        return std::current_exception();
    }
}

void outer() {
    try {
        throw std::logic_error("inner");
    } catch (...) {
        std::throw_with_nested(std::runtime_error("outer"));
    }
}

int main() {
    std::exception_ptr p = capture();
    try {
        std::rethrow_exception(p);
    } catch (const std::exception& e) {
        std::printf("%s\n", e.what());
    }
    try {
        outer();
    } catch (const std::exception& e) {
        std::printf("%s", e.what());
        try {
            std::rethrow_if_nested(e);
        } catch (const std::exception& inner) {
            std::printf(" <- %s\n", inner.what());
        }
    }
    return 0;
}
