// Does an operator function get the code cl gives it?
//
// Every overloadable operator has a two-character code and the codes
// follow no pattern a reader could guess. A conversion function is `?B`
// with its target written where the return type goes.
//
// Defined out of line, so that cl emits every one whether or not it is
// used; an inline member is emitted only where a use is.

struct V {
    int n;
    V operator+(const V &o) const;
    V operator-(const V &o) const;
    V operator*(const V &o) const;
    V operator/(const V &o) const;
    V operator%(const V &o) const;
    V operator^(const V &o) const;
    V operator&(const V &o) const;
    V operator|(const V &o) const;
    V operator~() const;
    bool operator!() const;
    bool operator<(const V &o) const;
    bool operator>(const V &o) const;
    bool operator<=(const V &o) const;
    bool operator>=(const V &o) const;
    bool operator==(const V &o) const;
    bool operator!=(const V &o) const;
    V &operator+=(const V &o);
    V &operator-=(const V &o);
    V &operator*=(const V &o);
    V &operator/=(const V &o);
    V &operator%=(const V &o);
    V &operator^=(const V &o);
    V &operator&=(const V &o);
    V &operator|=(const V &o);
    V operator<<(int k) const;
    V operator>>(int k) const;
    V &operator<<=(int k);
    V &operator>>=(int k);
    bool operator&&(const V &o) const;
    bool operator||(const V &o) const;
    V &operator++();
    V operator++(int);
    V &operator--();
    V operator--(int);
    int operator,(const V &o) const;
    V *operator->();
    int operator()(int a) const;
    int operator[](int i) const;
    operator int() const;
    operator bool() const;
    V operator-() const;
    V *operator&();
};

V V::operator+(const V &o) const { return {n + o.n}; }
V V::operator-(const V &o) const { return {n - o.n}; }
V V::operator*(const V &o) const { return {n * o.n}; }
V V::operator/(const V &o) const { return {n / o.n}; }
V V::operator%(const V &o) const { return {n % o.n}; }
V V::operator^(const V &o) const { return {n ^ o.n}; }
V V::operator&(const V &o) const { return {n & o.n}; }
V V::operator|(const V &o) const { return {n | o.n}; }
V V::operator~() const { return {~n}; }
bool V::operator!() const { return !n; }
bool V::operator<(const V &o) const { return n < o.n; }
bool V::operator>(const V &o) const { return n > o.n; }
bool V::operator<=(const V &o) const { return n <= o.n; }
bool V::operator>=(const V &o) const { return n >= o.n; }
bool V::operator==(const V &o) const { return n == o.n; }
bool V::operator!=(const V &o) const { return n != o.n; }
V &V::operator+=(const V &o) { n += o.n; return *this; }
V &V::operator-=(const V &o) { n -= o.n; return *this; }
V &V::operator*=(const V &o) { n *= o.n; return *this; }
V &V::operator/=(const V &o) { n /= o.n; return *this; }
V &V::operator%=(const V &o) { n %= o.n; return *this; }
V &V::operator^=(const V &o) { n ^= o.n; return *this; }
V &V::operator&=(const V &o) { n &= o.n; return *this; }
V &V::operator|=(const V &o) { n |= o.n; return *this; }
V V::operator<<(int k) const { return {n << k}; }
V V::operator>>(int k) const { return {n >> k}; }
V &V::operator<<=(int k) { n <<= k; return *this; }
V &V::operator>>=(int k) { n >>= k; return *this; }
bool V::operator&&(const V &o) const { return n && o.n; }
bool V::operator||(const V &o) const { return n || o.n; }
V &V::operator++() { ++n; return *this; }
V V::operator++(int) { V old = *this; ++n; return old; }
V &V::operator--() { --n; return *this; }
V V::operator--(int) { V old = *this; --n; return old; }
int V::operator,(const V &o) const { return o.n; }
V *V::operator->() { return this; }
int V::operator()(int a) const { return n + a; }
int V::operator[](int i) const { return n + i; }
V::operator int() const { return n; }
V::operator bool() const { return n != 0; }
V V::operator-() const { return {-n}; }
V *V::operator&() { return this; }

V operator+(int a, const V &b) { return {a + b.n}; }
bool operator<(int a, const V &b) { return a < b.n; }
