// Classes implementing several abstract interfaces, used through each.
#include <cstdio>

struct Readable {
    virtual int read() = 0;
    virtual ~Readable() = default;
};

struct Writable {
    virtual void write(int v) = 0;
    virtual ~Writable() = default;
};

struct Pipe : Readable, Writable {
    int buf[8];
    int head = 0, tail = 0;
    void write(int v) override { buf[tail++ % 8] = v; }
    int read() override { return buf[head++ % 8]; }
};

void produce(Writable& w) {
    for (int i = 1; i <= 3; ++i) w.write(i * i);
}

int consume(Readable& r) { return r.read() + r.read() * 10 + r.read() * 100; }

int main() {
    Pipe p;
    produce(p);
    std::printf("%d\n", consume(p));
    return 0;
}
