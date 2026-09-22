// A class owning heap memory: deep copy, assignment, destruction.
#include <cstdio>
#include <cstring>

class Str {
public:
    Str(const char* s) : p_(new char[std::strlen(s) + 1]) { std::strcpy(p_, s); }
    Str(const Str& o) : Str(o.p_) {}
    Str& operator=(const Str& o) {
        if (this != &o) {
            char* n = new char[std::strlen(o.p_) + 1];
            std::strcpy(n, o.p_);
            delete[] p_;
            p_ = n;
        }
        return *this;
    }
    ~Str() { delete[] p_; }
    void upper() { for (char* c = p_; *c; ++c) if (*c >= 'a' && *c <= 'z') *c -= 32; }
    const char* c_str() const { return p_; }

private:
    char* p_;
};

int main() {
    Str a("hello");
    Str b = a;
    b.upper();
    Str c("x");
    c = b;
    c = c;
    std::printf("%s %s %s\n", a.c_str(), b.c_str(), c.c_str());
    return 0;
}
