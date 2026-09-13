// The narrow types are held in a wide register and stored at their own
// width, so what a program observes is the truncation on the way to memory
// -- §7.3.9/2 -- and the sign or zero extension on the way back.
int main() {
    char c = 100;
    signed char sc = -100;
    unsigned char uc = 200;
    short s = 30000;
    unsigned short us = 60000;

    int total = 0;
    total = total + c;
    total = total + sc;
    total = total + uc;
    total = total + s / 100;
    total = total + us / 1000;
    return total % 251;
}
