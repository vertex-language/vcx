// Does a string literal live somewhere, and does its address travel?
//
// §5.13.5 -- a string literal is an array of const char with static
// storage duration, so `"hi"` is bytes in the read-only data and the
// expression is their address. The program reads them back through a
// pointer, through a subscript on the literal itself, and through a
// function declared in C linkage from the platform's own library -- the
// first import from outside the program.

extern "C" int strlen(const char *);

int count(const char *s) {
    int n = 0;
    while (*s++) n++;
    return n;
}

int main() {
    const char *greeting = "hello, world";
    char third = "abc"[2];
    return count(greeting) + strlen("four") + third - 'c';   // 12 + 4 + 0
}
