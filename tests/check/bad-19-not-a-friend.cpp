// §11.8.4 [class.friend]/2 -- friendship is neither inherited nor
// transitive, so a class the grant did not name has no access.
class Box {
    int shut = 1;
    friend class Inspector;
};

struct Inspector { int look(const Box& b) { return b.shut; } };

struct Impostor { int look(const Box& b) { return b.shut; } };
