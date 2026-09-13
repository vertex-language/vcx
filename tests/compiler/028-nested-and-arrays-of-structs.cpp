// A class inside a class, and an array of them: the offsets compose, which
// is the whole of what a layout is for.
struct Inner { int a; int b; };
struct Outer { Inner first; int mid; Inner second; };

int main() {
    Outer o;
    o.first.a = 1;
    o.first.b = 2;
    o.mid = 3;
    o.second.a = 4;
    o.second.b = 5;

    Inner pts[3];
    for (int i = 0; i < 3; ++i) {
        pts[i].a = i;
        pts[i].b = i * 10;
    }

    int total = o.first.a + o.first.b + o.mid + o.second.a + o.second.b;
    for (int i = 0; i < 3; ++i) total = total + pts[i].a + pts[i].b;
    return total;
}
