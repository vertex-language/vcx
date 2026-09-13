// §5.10 [lex.name] -- identifiers, and the two things that look like them.

int camelCase = 1;
int _underscoreStart = 2;
int mixed123 = 3;
int __reserved_to_the_implementation = 4;

// An identifier may hold characters outside the basic set: §5.10/1 defers to
// UAX #31, so these are identifiers and not an error to be recovered from.
int café = 5;
int Ω = 6;
int _Ⅻ = 7;

// A universal-character-name spells the same thing in the basic character set.
int Ångström = 8;

// §5.11 [lex.key] -- a keyword is not an identifier, but a keyword-like name
// is: none of these is on the table in §5.11.
int override = 9;
int final = 10;
int module = 11;
int import = 12;

// §5.10/3 -- an identifier is not a keyword no matter where it appears, and
// these contextual keywords are only keywords in their grammar position.
struct Contextual {
    int f() const;
};
int Contextual::f() const { return override + final; }
