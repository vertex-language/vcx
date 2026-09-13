struct Made {
    int n;
    Made(int k) : n(k) {}
};

struct Counter {
    int count;
    int step;
    Counter(int start, int step);
    void bump();
    int get() const;
    static int twice(int v);
    Made snapshot() const;
};

Counter::Counter(int start, int s) : count(start), step(s) {}
void Counter::bump() { count += step; }
int Counter::get() const { return count; }
int Counter::twice(int v) { return v * 2; }
Made Counter::snapshot() const { return Made(count); }
