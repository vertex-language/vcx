// A closing program: a tiny bank ledger using the later rungs together -- std::expected,
// ranges, std::format, a coroutine, deducing this, concepts and pointers to members.
#include <algorithm>
#include <concepts>
#include <coroutine>
#include <cstdio>
#include <exception>
#include <expected>
#include <format>
#include <map>
#include <ranges>
#include <string>
#include <vector>

struct Tx {
    std::string account;
    long cents;
};

template <typename T>
concept Amount = std::integral<T> && sizeof(T) >= 4;

struct Ledger {
    std::map<std::string, long> balances;
    std::vector<Tx> history;

    template <Amount A>
    std::expected<long, std::string> post(this Ledger& self, const std::string& who, A cents) {
        long next = self.balances[who] + cents;
        if (next < 0) return std::unexpected(std::format("{} would go to {}", who, next));
        self.history.push_back({who, cents});
        return self.balances[who] = next;
    }
};

struct Ids {
    struct promise_type {
        int v = 0;
        Ids get_return_object() { return Ids{std::coroutine_handle<promise_type>::from_promise(*this)}; }
        std::suspend_always initial_suspend() noexcept { return {}; }
        std::suspend_always final_suspend() noexcept { return {}; }
        std::suspend_always yield_value(int x) noexcept {
            v = x;
            return {};
        }
        void return_void() noexcept {}
        void unhandled_exception() { std::terminate(); }
    };
    std::coroutine_handle<promise_type> h;
    ~Ids() { h.destroy(); }
    int next() {
        h.resume();
        return h.promise().v;
    }
};

Ids ids() {
    for (int i = 1000;; ++i) co_yield i;
}

int main() {
    Ledger l;
    Ids gen = ids();
    const Tx script[] = {{"ann", 500}, {"bob", 300}, {"ann", -200}, {"bob", -400}, {"cat", 50}, {"ann", -300}};
    for (const Tx& t : script) {
        int id = gen.next();
        auto r = l.post(t.account, t.cents);
        std::printf("%s\n", r ? std::format("#{} {} -> {}", id, t.account, *r).c_str()
                              : std::format("#{} refused: {}", id, r.error()).c_str());
    }
    long Tx::*amount = &Tx::cents;
    auto deposits = l.history | std::views::filter([&](const Tx& t) { return t.*amount > 0; });
    long total = 0;
    for (const Tx& t : deposits) total += t.*amount;
    auto richest = std::ranges::max_element(l.balances, {}, &std::pair<const std::string, long>::second);
    std::printf("%s\n", std::format("deposits {} richest {} ({})", total, richest->first, richest->second).c_str());
    return 0;
}
