// std::priority_queue, std::stack and std::queue.
#include <cstdio>
#include <functional>
#include <queue>
#include <stack>
#include <vector>

int main() {
    std::priority_queue<int> max_heap;
    std::priority_queue<int, std::vector<int>, std::greater<int>> min_heap;
    for (int v : {5, 1, 8, 3, 9, 2}) {
        max_heap.push(v);
        min_heap.push(v);
    }
    for (int i = 0; i < 3; ++i) {
        std::printf("%d%d ", max_heap.top(), min_heap.top());
        max_heap.pop();
        min_heap.pop();
    }
    std::stack<int> s;
    std::queue<int> q;
    for (int i = 1; i <= 4; ++i) {
        s.push(i);
        q.push(i);
    }
    std::printf("| %d %d %zu\n", s.top(), q.front(), q.size());
    return 0;
}
