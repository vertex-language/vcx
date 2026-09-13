// Does a member function get the name cl gives it?
//
// The access, whether it is static or virtual, the cv on `this`, and the
// class in every parameter and return: the scheme spells all of it, and a
// class name already written is a back-reference the second time. The copy
// constructor is the canonical case -- `??0Widget@@QEAA@AEBU0@@Z` refers to
// the class by digit.
//
// The members are defined out of line. One defined in the class is inline,
// and cl emits an inline function only where it is used; a definition
// outside the class is emitted whether or not anything calls it, which
// makes the object a list of names rather than a list of uses.

struct Widget {
    int x;
    Widget();
    Widget(int v);
    Widget(const Widget &o);
    ~Widget();
    void set(int v);
    int get() const;
    void both(Widget, const Widget &, Widget *);
    static int count();
    Widget clone() const;
    const Widget &self() const;
    Widget &operator=(const Widget &o);
private:
    void hidden();
    static void hidden_static();
protected:
    void guarded();
public:
    void use_privates();
};

Widget::Widget() : x(0) {}
Widget::Widget(int v) : x(v) {}
Widget::Widget(const Widget &o) : x(o.x) {}
Widget::~Widget() {}
void Widget::set(int v) { x = v; }
int Widget::get() const { return x; }
void Widget::both(Widget, const Widget &, Widget *) {}
int Widget::count() { return 0; }
Widget Widget::clone() const { return *this; }
const Widget &Widget::self() const { return *this; }
Widget &Widget::operator=(const Widget &o) { x = o.x; return *this; }
void Widget::hidden() {}
void Widget::hidden_static() {}
void Widget::guarded() {}
void Widget::use_privates() { hidden(); hidden_static(); guarded(); }

class Shape {
public:
    Shape();
    virtual int area() const;
    virtual void scale(double);
    virtual ~Shape();
    int id();
};

Shape::Shape() {}
int Shape::area() const { return 0; }
void Shape::scale(double) {}
Shape::~Shape() {}
int Shape::id() { return 1; }

union U { int i; float f; void set(int v); };
void U::set(int v) { i = v; }

enum Color { Red, Green };
enum class Mode : unsigned char { A, B };

void takes(Widget) {}
void takes_all(Widget, Shape &, U *, Color, Mode) {}
Widget returns() { return Widget(); }
const Widget returns_const() { return Widget(); }
Color color() { return Red; }
Mode mode() { return Mode::A; }
U *unions() { return nullptr; }

// Shape declares its default constructor because cl would otherwise
// synthesize one as a function -- a polymorphic class's implicit
// constructor is not trivial, it installs the table pointer -- and vcx does
// that work inline where the object is declared. Whether that difference
// matters is a question for tests/compiler, where a program is linked from
// both compilers' objects; here the file asks only about names.
int main() {
    Shape s;
    return s.id();
}
