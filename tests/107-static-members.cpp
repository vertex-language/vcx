// Static data members and static member functions.
#include <cstdio>

class Widget {
public:
    Widget() { ++live; ++made; }
    ~Widget() { --live; }
    static int created() { return made; }
    static int live;

private:
    static int made;
};

int Widget::live = 0;
int Widget::made = 0;

int main() {
    {
        Widget a, b, c;
        std::printf("%d ", Widget::live);
    }
    Widget d;
    std::printf("%d %d\n", Widget::live, Widget::created());
    return 0;
}
