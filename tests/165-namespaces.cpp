// Namespaces: nested, reopened, aliased, and using-directives.
#include <cstdio>

namespace geo {
int scale = 2;
namespace flat {
int area(int w, int h) { return w * h * scale; }
}
}  // namespace geo

namespace geo {
int perimeter(int w, int h) { return 2 * (w + h); }
}

namespace a::b::c {
int deep() { return 3; }
}

namespace g = geo::flat;

int main() {
    using namespace geo;
    std::printf("%d %d %d\n", g::area(3, 4), perimeter(3, 4), a::b::c::deep());
    return 0;
}
