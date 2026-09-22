// An array of structs in a device buffer: members read and written.
//! kernel particles
//! grid 128 32
//! buffer float 512 rand
#include <metal_stdlib>
using namespace metal;

struct Particle {
    float x, y;
    float vx, vy;
};

kernel void particles(device Particle* p [[buffer(0)]], uint i [[thread_position_in_grid]]) {
    Particle q = p[i];
    q.x += q.vx * 0.5f;
    q.y += q.vy * 0.5f;
    if (q.x > 1.0f) {
        q.x = 2.0f - q.x;
        q.vx = -q.vx;
    }
    p[i] = q;
}
