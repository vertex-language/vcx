// A switch's condition is promoted, and a long, a size_t or an enum over
// one is 64 bits wide: the labels are compared at that width, so one past
// 32 bits is still told apart from its low half.
#include <cstddef>

enum class Kind : unsigned long { small = 0x200, wide = 0x100000200ul };

int classify(std::size_t kind) {
    switch (kind) {
    case 0x200:
        return 1;
    case 0x201:
    case 0x202:
        return 2;
    case 0x100000200ul:
        return 3;
    default:
        return 4;
    }
}

int ofKind(Kind k) {
    switch (k) {
    case Kind::small:
        return 10;
    case Kind::wide:
        return 20;
    }
    return 30;
}

int main() {
    int failures = 0;
    if (classify(0x200) != 1) failures++;
    if (classify(0x202) != 2) failures++;
    if (classify(0x100000200ul) != 3) failures++;
    if (classify(7) != 4) failures++;
    long negative = -5;
    switch (negative) {
    case -5: break;
    default: failures++;
    }
    if (ofKind(Kind::wide) != 20 || ofKind(Kind::small) != 10) failures++;
    return failures;
}
