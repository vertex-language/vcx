// std::sort with a lambda comparator over structs.
#include <algorithm>
#include <cstdio>
#include <vector>

struct Player {
    const char* name;
    int score;
};

int main() {
    std::vector<Player> p{{"ann", 30}, {"bob", 50}, {"cat", 30}, {"dan", 10}};
    std::sort(p.begin(), p.end(), [](const Player& a, const Player& b) {
        return a.score != b.score ? a.score > b.score : a.name[0] < b.name[0];
    });
    for (const Player& x : p) std::printf("%s:%d ", x.name, x.score);
    std::printf("\n");
    return 0;
}
