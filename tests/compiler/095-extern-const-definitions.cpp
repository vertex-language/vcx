// A const object at namespace scope has internal linkage alone, but one
// declared extern first keeps the external linkage of that declaration
// ([dcl.stc]/7), braced initializer and all. And an __asm label names an
// object's symbol exactly, so a second declaration under another name,
// labelled with the first's symbol, is the same object.
struct Descriptor {
    unsigned flags;
    int parent;
    int name;
};

extern "C" {
extern const Descriptor shared;
const Descriptor shared = {3, 5, 7};

extern const Descriptor labelled __asm("_vcx_test_labelled_descriptor");
const Descriptor labelled = {11, 13, 17};

extern const Descriptor same_as_shared __asm("_shared");
extern const Descriptor same_as_labelled __asm("_vcx_test_labelled_descriptor");
}

int main() {
    int failures = 0;
    if (shared.flags != 3 || shared.parent != 5 || shared.name != 7) failures |= 1;
    if (labelled.flags != 11 || labelled.name != 17) failures |= 2;
    if (same_as_shared.parent != 5) failures |= 4;
    if (same_as_labelled.parent != 13) failures |= 8;
    return failures;
}
