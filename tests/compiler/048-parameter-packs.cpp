// Do function parameter packs expand into code?
//
// §13.7.4 -- `Ts &&... vs` is as many parameters as the call supplies,
// each deduced on its own (a forwarding reference per element), and
// `vs...` in the body is those parameters again, once per element, with
// the pattern around each -- `forward<Ts>(vs)...` is the forward of each.
// `sizeof...(vs)` is their number. The object-file names spell the pack's
// arguments one after another, or `$$V` for none, which is how cl spells
// `count<>` (`??$count@$$V@@YAHXZ`).
//
// With them: an anonymous union (§11.5.2), whose members are the
// enclosing class's and share storage; and a class declared before it
// is defined (§6.8.1/8), which a pointer may name in between.

template <class T> struct remove_reference { using type = T; };
template <class T> struct remove_reference<T&> { using type = T; };
template <class T> struct remove_reference<T&&> { using type = T; };
template <class T> using remove_reference_t = typename remove_reference<T>::type;
template <class T> constexpr T&& forward(remove_reference_t<T>& arg) noexcept { return static_cast<T&&>(arg); }
template <class T> constexpr T&& forward(remove_reference_t<T>&& arg) noexcept { return static_cast<T&&>(arg); }

template <class... Ts> int count(Ts&&... vs) { return sizeof...(vs); }
int sum3(int a, int b, int c) { return a + b + c; }
template <class... Ts> int sum(Ts&&... vs) { return sum3(forward<Ts>(vs)...); }

struct Node;
struct Ref { Node* target; int weight; };
struct Node { int id; Ref next; };

struct Cell {
    int tag;
    union {
        int i;
        double d;
    };
    template <class... Args> Cell(int t, Args&&... args) : tag(t) { set(forward<Args>(args)...); }
    void set(int v) { i = v; }
    void set(double v) { d = v; }
    void set() { i = 0; }
    int get() const { return tag == 0 ? i : (int)d; }
};

struct Pair { int first, second; Pair(int a, int b) : first(a), second(b) {} };
template <class T, class... Args> T make(Args&&... args) { return T(forward<Args>(args)...); }

int main() {
    int x = 4;
    Node b{2, {nullptr, 0}};
    Node a{1, {&b, 7}};
    Cell c0(0, 40), c1(1, 2.5), c2(0);
    Pair p = make<Pair>(x, 5);
    return count() * 100 + count(1, 2.0, x) * 10 + sum(1, 2, 3) + a.next.target->id + a.next.weight
         + c0.get() + c1.get() + c2.get() + p.first + p.second + (sizeof(Cell) == 16 ? 0 : 1000);
    // 0 + 30 + 6 + 2 + 7 + 40 + 2 + 0 + 4 + 5 = 96
}
