// A switch over far-apart cases.
#include <cstdio>

const char* name(int code) {
    switch (code) {
    case 200: return "ok";
    case 404: return "not found";
    case 500: return "error";
    case -1000000: return "far";
    }
    return "?";
}

int main() {
    std::printf("%s %s %s %s %s\n", name(200), name(404), name(500), name(-1000000), name(3));
    return 0;
}
