// An array of structs, sorted by a field.
#include <cstdio>

struct Item {
    const char* name;
    int weight;
};

int main() {
    Item items[] = {{"c", 30}, {"a", 10}, {"d", 40}, {"b", 20}};
    for (int i = 0; i < 4; ++i)
        for (int j = i + 1; j < 4; ++j)
            if (items[j].weight < items[i].weight) {
                Item t = items[i];
                items[i] = items[j];
                items[j] = t;
            }
    for (const Item& it : items) std::printf("%s", it.name);
    std::printf("\n");
    return 0;
}
