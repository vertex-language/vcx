// Does a member template's specialization, and a pack's, get the name cl
// gives it?
//
// A constructor template's specialization keeps the constructor's code
// inside the template-name unit: `??$?0N@P@@QEAA@AEBN@Z` is P::P<double>.
// A method template's is `??$add@N@P@@`, a static one's `??$both@HJ@P@@`
// -- `J` for long, which `2L` is on this target. A parameter pack's
// arguments are written one after another as if each were its own, and
// an empty pack is `$$V`: `??$count@$$V@@YAHXZ` is count<>().

struct P {
    int a;
    template <class U> P(const U& x) : a((int)x) {}
    template <class U> int add(U v) const { return a + (int)v; }
    template <class U, class V> static int both(U u, V v) { return (int)u + (int)v; }
};
template <class T> struct Q {
    T t;
    template <class U> Q(const U& x) : t((T)x) {}
    template <class U> T get(U v) const { return t + (T)v; }
};
template <class... Ts> int count(Ts&&... vs) { return sizeof...(vs); }
template <class... Ts> int cnt2(Ts... vs) { return sizeof...(vs); }
template <class T, class... Ts> int lead(T t, Ts... vs) { return sizeof...(vs); }

int main() {
    P p(2.5);
    Q<int> q(1);
    int x = 1;
    return p.add(1.5) + P::both(1, 2L) + q.get('a') + count() + count(1, 2.0, x) + cnt2() + cnt2(1, 'c') + lead(1) + lead(1, 2);
}
