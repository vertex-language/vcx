// Do bit-fields hold their bits, and unions their bytes?
//
// §11.4.10 [class.bit] -- a bit-field is width bits of a storage unit,
// packed as the platform packs them (cl: from the least significant bit,
// a new unit when the declared type changes -- tests/abi settles where);
// reading one is its value sign-extended for a signed type, writing one
// touches its bits and no others. §11.5 [class.union] -- a union's members
// share storage, and reading a byte array over an int reads its bytes in
// the machine's order.

struct Flags {
    unsigned a : 3;
    unsigned b : 5;
    int c : 4;              // signed: -8..7
    unsigned : 0;           // a new unit follows
    unsigned d : 1;
};

struct Wide {
    unsigned long long lo : 40;
    unsigned long long hi : 24;
};

struct Bytes {
    unsigned char x : 4;
    unsigned char y : 4;
};

union U {
    int i;
    float f;
    unsigned char b[4];
};

union Tagged {
    struct { int kind; int v; } iv;
    struct { int kind; double d; } dv;
};

int main() {
    int r = 0;

    Flags f;
    f.a = 5; f.b = 17; f.c = -3; f.d = 1;
    if (f.a == 5 && f.b == 17 && f.c == -3 && f.d == 1) r += 1;

    // Writing one leaves the others alone; arithmetic wraps to the width.
    f.b = 0;
    f.a += 4;               // 9 in 3 bits is 1
    f.c = 7;
    f.c += 1;               // 8 in a signed 4-bit field is -8
    if (f.a == 1 && f.b == 0 && f.c == -8 && f.d == 1) r += 1;

    // Increments and compound assignments read and write the field.
    Bytes by;
    by.x = 14; by.y = 3;
    ++by.x;                 // 15
    by.y <<= 2;             // 12
    by.x++;                 // 16 wraps to 0
    if (by.x == 0 && by.y == 12 && sizeof(Bytes) == 1) r += 1;

    // A 64-bit unit.
    Wide w;
    w.lo = 0xFFFFFFFFFFull;
    w.hi = 0x123456;
    w.lo -= 1;
    if (w.lo == 0xFFFFFFFFFEull && w.hi == 0x123456 && sizeof(Wide) == 8) r += 1;

    // Reading through a pointer and a reference.
    Flags* pf = &f;
    Flags& rf = f;
    pf->a = 6;
    rf.c = -1;
    if (f.a == 6 && pf->c == -1 && rf.d == 1) r += 1;

    // A union's members share their bytes.
    U u;
    u.i = 0x01020304;
    if (u.b[0] == 4 && u.b[3] == 1 && sizeof(U) == 4) r += 1;
    u.f = 1.0f;
    if (u.i == 0x3F800000) r += 1;

    // A common initial sequence read through the other member.
    Tagged t;
    t.dv.kind = 2;
    t.dv.d = 2.5;
    if (t.iv.kind == 2 && sizeof(Tagged) == 16) r += 1;

    return r; // eight checks
}
