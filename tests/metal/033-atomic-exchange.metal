// Atomic exchange and compare-exchange: one thread claims each slot.
//! kernel k
//! grid 1024 256
//! buffer uint 16 zero
//! buffer uint 16 zero
//! buffer uint 1 zero
#include <metal_stdlib>
using namespace metal;

kernel void k(device atomic_uint* owner [[buffer(0)]], device atomic_uint* last [[buffer(1)]],
              device atomic_uint* claims [[buffer(2)]], uint i [[thread_position_in_grid]]) {
    uint slot = i % 16;
    // A weak compare-exchange may fail spuriously, so retry while the slot is still free.
    uint expected = 0;
    bool won = false;
    while (!(won = atomic_compare_exchange_weak_explicit(&owner[slot], &expected, 1u, memory_order_relaxed,
                                                         memory_order_relaxed)) &&
           expected == 0) {
    }
    if (won)
        atomic_fetch_add_explicit(claims, 1u, memory_order_relaxed);
    atomic_exchange_explicit(&last[slot], 7u, memory_order_relaxed);
}
