// §7.6.14 and §7.6.15 -- && and || short-circuit, which is a sequencing
// guarantee rather than an optimization: the right operand of a false && is
// not evaluated, and a program may rely on being spared it.
int side_effect(int* counter, int result) {
    *counter = *counter + 1;
    return result;
}

int main() {
    int calls = 0;
    int a = (0 && side_effect(&calls, 1));
    int b = (1 || side_effect(&calls, 1));
    int c = (1 && side_effect(&calls, 1));
    int d = (0 || side_effect(&calls, 1));
    return a + b * 2 + c * 4 + d * 8 + calls * 16;
}
