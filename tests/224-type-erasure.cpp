// Type erasure: any type with draw() held behind one interface.
#include <cstdio>
#include <memory>
#include <vector>

class Drawable {
    struct Concept {
        virtual ~Concept() = default;
        virtual int draw() const = 0;
    };
    template <typename T>
    struct Model : Concept {
        T obj;
        explicit Model(T o) : obj(o) {}
        int draw() const override { return obj.draw(); }
    };
    std::unique_ptr<Concept> self_;

public:
    template <typename T>
    Drawable(T obj) : self_(std::make_unique<Model<T>>(obj)) {}
    int draw() const { return self_->draw(); }
};

struct Circle {
    int r;
    int draw() const { return r * 3; }
};
struct Text {
    const char* s;
    int draw() const {
        int n = 0;
        while (s[n]) ++n;
        return n;
    }
};

int main() {
    std::vector<Drawable> scene;
    scene.emplace_back(Circle{5});
    scene.emplace_back(Text{"hello"});
    scene.emplace_back(Circle{1});
    int total = 0;
    for (const Drawable& d : scene) total += d.draw();
    std::printf("%d\n", total);
    return 0;
}
