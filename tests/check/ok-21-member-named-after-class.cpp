// §11.4.1 [class.mem.general] -- a non-static data member may have the
// name of its class, provided the class has no user-declared constructor.
// The member hides the injected-class-name for ordinary lookup, and an
// elaborated-type-specifier still names the class. Darwin's <netinet/in.h>
// is written against exactly this: struct ip_opts { ... char ip_opts[40]; }.

struct in_addr { unsigned s_addr; };

struct ip_opts {
    struct in_addr ip_dst;
    char ip_opts[40];
};

struct Counter { int Counter; };

int main() {
    struct ip_opts o{};
    o.ip_opts[0] = 3;
    Counter c{4};
    struct Counter *p = &c;
    return o.ip_opts[0] + p->Counter + static_cast<int>(sizeof(o.ip_opts));
}
