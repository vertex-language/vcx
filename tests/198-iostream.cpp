// std::cout with the usual manipulators.
#include <iomanip>
#include <iostream>

int main() {
    std::cout << "sum " << 2 + 3 << ' ' << 1.5 << std::endl;
    std::cout << std::hex << 255 << std::dec << ' ' << std::setw(5) << 42 << '|' << std::endl;
    std::cout << std::fixed << std::setprecision(3) << 3.14159 << "\n";
    return 0;
}
