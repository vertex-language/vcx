// 052-enums.cpp
// Tests unscoped and scoped enums (enum class):
// - unscoped enum with custom and default values
// - scoped enum with specified underlying type
// - negative enumerator values
// - switch statements over enums
// - enum comparisons
// - enum member variables and parameters

enum Status {
    Ok = 0,
    Warning = 5,
    Error = 10
};

enum class Color : unsigned char {
    Red = 1,
    Green = 2,
    Blue = 4
};

enum class Delta : int {
    Down = -5,
    None = 0,
    Up = 5
};

struct Packet {
    Status status;
    Color color;
};

int check_status(Status s) {
    switch (s) {
    case Ok:
        return 100;
    case Warning:
        return 200;
    case Error:
        return 300;
    }
    return 0;
}

int check_color(Color c) {
    if (c == Color::Red) return 10;
    if (c == Color::Green) return 20;
    if (c == Color::Blue) return 30;
    return 0;
}

int check_delta(Delta d) {
    int v = 50;
    if (d == Delta::Down) v += -5;
    else if (d == Delta::Up) v += 5;
    return v;
}

int main() {
    Status s = Warning;
    int r1 = check_status(s); // 200

    Color c = Color::Blue;
    int r2 = check_color(c); // 30

    Delta d = Delta::Down;
    int r3 = check_delta(d); // 45

    Packet p;
    p.status = Ok;
    p.color = Color::Green;
    int r4 = check_status(p.status) + check_color(p.color); // 100 + 20 = 120

    // Total: 200 + 30 + 45 + 120 = 395
    // 395 % 256 = 139
    return (r1 + r2 + r3 + r4) % 256;
}
