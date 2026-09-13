// §6.7.5.2 -- an object with static storage duration, which lives outside
// every function and is zero-initialised before any of them runs.
int counter;
int seeded = 41;

int bump() {
    counter = counter + 1;
    return counter;
}

int main() {
    bump();
    bump();
    bump();
    return counter + seeded - 43;
}
