// Does <cstdint> compile, from the toolset this machine has?
//
// The first header from outside the program, and the one every other
// standard header reaches through <vcruntime.h>: it is the test of the
// sysroot, of the target's predefined macros, and of the Microsoft
// spellings the toolset's headers are written in -- __pragma, __declspec,
// __cdecl, __unaligned, #pragma pack -- before it is a test of anything
// C++.

#include <cstdint>

int main() {
    std::int32_t x = 5;
    uint64_t y = 6;
    std::int8_t small = -1;
    return x + (int)y + small + (int)sizeof(std::uintptr_t) - 18;   // 5 + 6 - 1 + 8 - 18 = 0
}
