// A small program using most of the ladder at once: a class hierarchy, templates,
// the standard library and lambdas, evaluating expressions from a token list.
#include <cstdio>
#include <map>
#include <memory>
#include <string>
#include <vector>

struct Expr {
    virtual long eval(const std::map<std::string, long>& env) const = 0;
    virtual ~Expr() = default;
};

struct Num : Expr {
    long v;
    explicit Num(long x) : v(x) {}
    long eval(const std::map<std::string, long>&) const override { return v; }
};

struct Var : Expr {
    std::string name;
    explicit Var(std::string n) : name(std::move(n)) {}
    long eval(const std::map<std::string, long>& env) const override { return env.at(name); }
};

template <typename Op>
struct Bin : Expr {
    std::unique_ptr<Expr> l, r;
    Bin(std::unique_ptr<Expr> a, std::unique_ptr<Expr> b) : l(std::move(a)), r(std::move(b)) {}
    long eval(const std::map<std::string, long>& env) const override { return Op{}(l->eval(env), r->eval(env)); }
};

struct Add {
    long operator()(long a, long b) const { return a + b; }
};
struct Mul {
    long operator()(long a, long b) const { return a * b; }
};

// Reverse Polish: "x 3 * y +" is x*3 + y.
std::unique_ptr<Expr> parse(const std::vector<std::string>& tokens) {
    std::vector<std::unique_ptr<Expr>> stack;
    for (const std::string& t : tokens) {
        if (t == "+" || t == "*") {
            auto r = std::move(stack.back());
            stack.pop_back();
            auto l = std::move(stack.back());
            stack.pop_back();
            if (t == "+") stack.push_back(std::make_unique<Bin<Add>>(std::move(l), std::move(r)));
            else stack.push_back(std::make_unique<Bin<Mul>>(std::move(l), std::move(r)));
        } else if (t[0] >= '0' && t[0] <= '9') {
            stack.push_back(std::make_unique<Num>(std::stol(t)));
        } else {
            stack.push_back(std::make_unique<Var>(t));
        }
    }
    return std::move(stack.back());
}

int main() {
    auto e = parse({"x", "3", "*", "y", "+", "2", "*"});
    std::map<std::string, long> env{{"x", 5}, {"y", 7}};
    for (long x = 0; x < 3; ++x) {
        env["x"] = x;
        std::printf("%ld ", e->eval(env));
    }
    std::printf("\n");
    return 0;
}
