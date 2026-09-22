// Pointer arithmetic over device memory: offsets, differences, walking a row.
//! kernel k
//! grid 64 16
//! buffer int 1024 rand
//! buffer int 64 zero
#include <metal_stdlib>
using namespace metal;

int row_max(device const int* row, int n) {
    device const int* end = row + n;
    int best = *row;
    for (device const int* p = row + 1; p < end; ++p)
        best = max(best, *p);
    return best;
}

kernel void k(device const int* m [[buffer(0)]], device int* out [[buffer(1)]],
              uint i [[thread_position_in_grid]]) {
    device const int* row = m + i * 16;
    out[i] = row_max(row, 16) / 2 + int(&row[15] - row);
}
