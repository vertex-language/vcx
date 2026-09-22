// A coroutine generator written against <coroutine> directly: co_yield, suspension, destruction.
#include <coroutine>
#include <cstdio>
#include <exception>

struct Generator {
    struct promise_type {
        int current = 0;
        Generator get_return_object() { return Generator{std::coroutine_handle<promise_type>::from_promise(*this)}; }
        std::suspend_always initial_suspend() noexcept { return {}; }
        std::suspend_always final_suspend() noexcept { return {}; }
        std::suspend_always yield_value(int v) noexcept {
            current = v;
            return {};
        }
        void return_void() noexcept {}
        void unhandled_exception() { std::terminate(); }
    };

    explicit Generator(std::coroutine_handle<promise_type> h) : h_(h) {}
    Generator(Generator&& o) noexcept : h_(o.h_) { o.h_ = nullptr; }
    ~Generator() {
        if (h_) h_.destroy();
    }
    bool next() {
        h_.resume();
        return !h_.done();
    }
    int value() const { return h_.promise().current; }

private:
    std::coroutine_handle<promise_type> h_;
};

Generator fibonacci(int n) {
    int a = 0, b = 1;
    for (int i = 0; i < n; ++i) {
        co_yield a;
        int t = a + b;
        a = b;
        b = t;
    }
}

int main() {
    Generator g = fibonacci(12);
    while (g.next()) std::printf("%d ", g.value());
    std::printf("\n");
    return 0;
}
