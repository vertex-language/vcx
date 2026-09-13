# C++ Language Grammar

This document presents the grammar for the C++ programming language, expressed using the notation of the Java Language Specification (JLS) — a BNF variant chosen for being shorter and easier to read than other common styles.

**Source:** ISO/IEC JTC1/SC22/WG21 working draft N5054 (2026-07-16), Annex A (Grammar summary). C++26 was published by WG21 on 28 March 2026, following ballot-comment resolution at Kona in November 2025; this document reflects the post-C++26 working draft, which is grammatically near-identical to C++26 at the time of writing.

**Status note (from A.1):** This summary is an aid to comprehension, not an exact statement of the language. The grammar here accepts a *superset* of valid C++ constructs. Disambiguation rules ([stmt.ambig], [dcl.spec], [class.member.lookup]) distinguish expressions from declarations, and access control, ambiguity, and type rules weed out syntactically valid but meaningless constructs. Do not expect a parser built directly from this to reject all ill-formed programs.

## Notation

```text
X          a terminal or nonterminal
[X]        X is optional
{X}        zero or more repetitions of X
'X'        X is a literal character, quoted to distinguish it from
           the grouping metacharacters above
(one of)   the alternatives are listed inline rather than per-line
(prose)    a parenthesized English description of a terminal
```

Left recursion in the standard's rules (`List: Item | List , Item`) is rendered here as `Item {, Item}`. Ordering within a "one of" list carries no grammatical meaning.

Where the standard spells out a list of *optional* elements as separate alternatives (as it does for `attribute-list`), this document collapses it to the shorter `[X] {, [X]}` form. The accepted language is the same, but the rendering is not a production-for-production match — do not back-translate it and expect to recover the standard's form.

## Lexical Structure

### Tokens

```text
Token:
    Identifier
    Keyword
    Literal
    HeaderName
    OperatorOrPunctuator

PreprocessingToken:
    HeaderName
    ImportKeyword
    ModuleKeyword
    ExportKeyword
    Identifier
    PpNumber
    CharacterLiteral
    UserDefinedCharacterLiteral
    StringLiteral
    UserDefinedStringLiteral
    PreprocessingOpOrPunc
    (each non-whitespace character that cannot be one of the above)
```

### Keywords

```text
Keyword:
    (any identifier listed in the keyword table below)
    ImportKeyword
    ModuleKeyword
    ExportKeyword

Keyword: (one of)
    alignas      alignof        asm          auto         bool
    break        case           catch        char         char8_t
    char16_t     char32_t       class        concept      const
    const_cast   consteval      constexpr    constinit    continue
    contract_assert  co_await   co_return    co_yield     decltype
    default      delete         do           double       dynamic_cast
    else         enum           explicit     export       extern
    false        float          for          friend       goto
    if           inline         int          long         mutable
    namespace    new            noexcept     nullptr      operator
    private      protected      public       register     reinterpret_cast
    requires     return         short        signed       sizeof
    static       static_assert  static_cast  struct       switch
    template     this           thread_local throw        true
    try          typedef        typeid       typename     union
    unsigned     using          virtual      void         volatile
    wchar_t      while
```

> **Alternative tokens.** `and`, `and_eq`, `bitand`, `bitor`, `compl`, `not`, `not_eq`, `or`, `or_eq`, `xor`, and `xor_eq` are *not* keywords. They are alternative tokens ([lex.digraph]) and appear in `OperatorOrPunctuator` below, as do the digraphs `<%`, `%>`, `<:`, and `:>`. The two remaining digraphs, `%:` and `%:%:`, are not `OperatorOrPunctuator`s — they appear in `PreprocessingOperator`, alongside `#` and `##`.

### Context-Dependent Keywords

New context-dependent keywords are introduced by `typedef`, `namespace`, class, enumeration, and `template` declarations.

```text
TypedefName:
    Identifier
    SimpleTemplateId

NamespaceName:
    Identifier
    NamespaceAlias

NamespaceAlias:
    Identifier

ClassName:
    Identifier
    SimpleTemplateId

EnumName:
    Identifier

SimpleTemplateName:
    Identifier
```

### Identifiers

```text
Identifier:
    IdentifierStart
    Identifier IdentifierContinue

IdentifierStart:
    Nondigit
    (an element of the translation character set with the Unicode property XID_Start or ID_Compat_Math_Start)

IdentifierContinue:
    Digit
    Nondigit
    (an element of the translation character set with the Unicode property XID_Continue or ID_Compat_Math_Continue)

Nondigit: (one of)
    a  b  c  d  e  f  g  h  i  j  k  l  m  n  o  p  q  r  s  t  u  v  w  x  y  z
    A  B  C  D  E  F  G  H  I  J  K  L  M  N  O  P  Q  R  S  T  U  V  W  X  Y  Z
    _

Digit: (one of)
    0  1  2  3  4  5  6  7  8  9
```

### Universal Character Names

```text
UniversalCharacterName:
    \u HexQuad
    \U HexQuad HexQuad
    \u '{' SimpleHexadecimalDigitSequence '}'
    NamedUniversalCharacter

NamedUniversalCharacter:
    \N '{' NCharSequence '}'

NCharSequence:
    NChar {NChar}

NChar:
    (any member of the translation character set except U+007D right curly bracket or new-line)

HexQuad:
    HexadecimalDigit HexadecimalDigit HexadecimalDigit HexadecimalDigit

SimpleHexadecimalDigitSequence:
    HexadecimalDigit {HexadecimalDigit}
```

### Header Names and Preprocessing Numbers

```text
HeaderName:
    < HCharSequence >
    " QCharSequence "

HCharSequence:
    HChar {HChar}

HChar:
    (any member of the translation character set except new-line and U+003E greater-than sign)

QCharSequence:
    QChar {QChar}

QChar:
    (any member of the translation character set except new-line and U+0022 quotation mark)

PpNumber:
    Digit
    . Digit
    PpNumber IdentifierContinue
    PpNumber ' Digit
    PpNumber ' Nondigit
    PpNumber e Sign
    PpNumber E Sign
    PpNumber p Sign
    PpNumber P Sign
    PpNumber .
```

### Literals

```text
Literal:
    IntegerLiteral
    CharacterLiteral
    FloatingPointLiteral
    StringLiteral
    BooleanLiteral
    PointerLiteral
    UserDefinedLiteral

IntegerLiteral:
    BinaryLiteral [IntegerSuffix]
    OctalLiteral [IntegerSuffix]
    DecimalLiteral [IntegerSuffix]
    HexadecimalLiteral [IntegerSuffix]

BinaryLiteral:
    0b BinaryDigit
    0B BinaryDigit
    BinaryLiteral ['] BinaryDigit

OctalLiteral:
    0
    OctalLiteral ['] OctalDigit

DecimalLiteral:
    NonzeroDigit
    DecimalLiteral ['] Digit

HexadecimalLiteral:
    HexadecimalPrefix HexadecimalDigitSequence

BinaryDigit: (one of)
    0  1

OctalDigit: (one of)
    0  1  2  3  4  5  6  7

NonzeroDigit: (one of)
    1  2  3  4  5  6  7  8  9

HexadecimalPrefix: (one of)
    0x  0X

HexadecimalDigitSequence:
    HexadecimalDigit
    HexadecimalDigitSequence ['] HexadecimalDigit

HexadecimalDigit: (one of)
    0  1  2  3  4  5  6  7  8  9  a  b  c  d  e  f  A  B  C  D  E  F

IntegerSuffix:
    UnsignedSuffix [LongSuffix]
    UnsignedSuffix [LongLongSuffix]
    UnsignedSuffix [SizeSuffix]
    LongSuffix [UnsignedSuffix]
    LongLongSuffix [UnsignedSuffix]
    SizeSuffix [UnsignedSuffix]

UnsignedSuffix: (one of)
    u  U

LongSuffix: (one of)
    l  L

LongLongSuffix: (one of)
    ll  LL

SizeSuffix: (one of)
    z  Z

CharacterLiteral:
    [EncodingPrefix] ' CCharSequence '

EncodingPrefix: (one of)
    u8  u  U  L

CCharSequence:
    CChar {CChar}

CChar:
    BasicCChar
    EscapeSequence
    UniversalCharacterName

BasicCChar:
    (any member of the translation character set except U+0027 apostrophe, U+005C reverse solidus, or new-line)

EscapeSequence:
    SimpleEscapeSequence
    NumericEscapeSequence
    ConditionalEscapeSequence

SimpleEscapeSequence:
    \ SimpleEscapeSequenceChar

SimpleEscapeSequenceChar: (one of)
    '  "  ?  \  a  b  f  n  r  t  v

NumericEscapeSequence:
    OctalEscapeSequence
    HexadecimalEscapeSequence

OctalEscapeSequence:
    \ OctalDigit
    \ OctalDigit OctalDigit
    \ OctalDigit OctalDigit OctalDigit
    \o '{' SimpleOctalDigitSequence '}'

SimpleOctalDigitSequence:
    OctalDigit {OctalDigit}

HexadecimalEscapeSequence:
    \x SimpleHexadecimalDigitSequence
    \x '{' SimpleHexadecimalDigitSequence '}'

ConditionalEscapeSequence:
    \ ConditionalEscapeSequenceChar

ConditionalEscapeSequenceChar:
    (any member of the basic character set that is not an OctalDigit, a SimpleEscapeSequenceChar, or one of N, o, u, U, x)

FloatingPointLiteral:
    DecimalFloatingPointLiteral
    HexadecimalFloatingPointLiteral

DecimalFloatingPointLiteral:
    FractionalConstant [ExponentPart] [FloatingPointSuffix]
    DigitSequence ExponentPart [FloatingPointSuffix]

HexadecimalFloatingPointLiteral:
    HexadecimalPrefix HexadecimalFractionalConstant BinaryExponentPart [FloatingPointSuffix]
    HexadecimalPrefix HexadecimalDigitSequence BinaryExponentPart [FloatingPointSuffix]

FractionalConstant:
    [DigitSequence] . DigitSequence
    DigitSequence .

HexadecimalFractionalConstant:
    [HexadecimalDigitSequence] . HexadecimalDigitSequence
    HexadecimalDigitSequence .

ExponentPart:
    e [Sign] DigitSequence
    E [Sign] DigitSequence

BinaryExponentPart:
    p [Sign] DigitSequence
    P [Sign] DigitSequence

Sign: (one of)
    +  -

DigitSequence:
    Digit
    DigitSequence ['] Digit

FloatingPointSuffix: (one of)
    f    l    f16  f32  f64  f128  bf16
    F    L    F16  F32  F64  F128  BF16

StringLiteral:
    [EncodingPrefix] PlainStringLiteral
    [EncodingPrefix] R RawString

PlainStringLiteral:
    " [SCharSequence] "

SCharSequence:
    SChar {SChar}

SChar:
    BasicSChar
    EscapeSequence
    UniversalCharacterName

BasicSChar:
    (any member of the translation character set except U+0022 quotation mark, U+005C reverse solidus, or new-line)

RawString:
    " [DCharSequence] ( [RCharSequence] ) [DCharSequence] "

RCharSequence:
    RChar {RChar}

RChar:
    (any member of the translation character set, except a U+0029 right parenthesis followed by the initial DCharSequence — which may be empty — followed by a U+0022 quotation mark)

DCharSequence:
    DChar {DChar}

DChar:
    (any member of the basic character set except U+0020 space, U+0028 left parenthesis, U+0029 right parenthesis, U+005C reverse solidus, U+0009 tab, U+000B vertical tab, U+000C form feed, and new-line)

UnevaluatedString:
    StringLiteral

BooleanLiteral:
    false
    true

PointerLiteral:
    nullptr

UserDefinedLiteral:
    UserDefinedIntegerLiteral
    UserDefinedFloatingPointLiteral
    UserDefinedStringLiteral
    UserDefinedCharacterLiteral

UserDefinedIntegerLiteral:
    DecimalLiteral UdSuffix
    OctalLiteral UdSuffix
    HexadecimalLiteral UdSuffix
    BinaryLiteral UdSuffix

UserDefinedFloatingPointLiteral:
    FractionalConstant [ExponentPart] UdSuffix
    DigitSequence ExponentPart UdSuffix
    HexadecimalPrefix HexadecimalFractionalConstant BinaryExponentPart UdSuffix
    HexadecimalPrefix HexadecimalDigitSequence BinaryExponentPart UdSuffix

UserDefinedStringLiteral:
    StringLiteral UdSuffix

UserDefinedCharacterLiteral:
    CharacterLiteral UdSuffix

UdSuffix:
    Identifier
```

### Operators and Punctuators

```text
PreprocessingOpOrPunc:
    PreprocessingOperator
    OperatorOrPunctuator

PreprocessingOperator: (one of)
    #  ##  %:  %:%:

OperatorOrPunctuator: (one of)
    {    }    [    ]    (    )    [:   :]
    <%   %>   <:   :>   ;    :    ...
    ?    ::   .    .*   ->   ->*  ^^   ~
    !    +    -    *    /    %    ^    &    |
    =    +=   -=   *=   /=   %=   ^=   &=   |=
    ==   !=   <    >    <=   >=   <=>  &&   ||
    <<   >>   <<=  >>=  ++   --   ,
    and  or   xor  not  bitand  bitor  compl
    and_eq  or_eq  xor_eq  not_eq
```

### Preprocessor-Produced Terminals

These three terminals are produced during preprocessing rather than by the lexical grammar proper; they are specified in [lex.pptoken].

```text
ImportKeyword
ModuleKeyword
ExportKeyword
```

## Basics

```text
TranslationUnit:
    [DeclarationSeq]
    [GlobalModuleFragment] ModuleDeclaration [DeclarationSeq] [PrivateModuleFragment]

SpliceSpecifier:
    '[:' ConstantExpression ':]'

SpliceSpecializationSpecifier:
    SpliceSpecifier < [TemplateArgumentList] >
```

## Expressions

```text
PrimaryExpression:
    Literal
    this
    ( Expression )
    IdExpression
    LambdaExpression
    FoldExpression
    RequiresExpression
    SpliceExpression

IdExpression:
    UnqualifiedId
    QualifiedId
    PackIndexExpression

UnqualifiedId:
    Identifier
    OperatorFunctionId
    ConversionFunctionId
    LiteralOperatorId
    ~ TypeName
    ~ ComputedTypeSpecifier
    TemplateId

QualifiedId:
    NestedNameSpecifier [template] UnqualifiedId

NestedNameSpecifier:
    ::
    TypeName ::
    NamespaceName ::
    ComputedTypeSpecifier ::
    SpliceScopeSpecifier ::
    NestedNameSpecifier Identifier ::
    NestedNameSpecifier [template] SimpleTemplateId ::

SpliceScopeSpecifier:
    SpliceSpecifier
    [template] SpliceSpecializationSpecifier

PackIndexExpression:
    IdExpression ... '[' ConstantExpression ']'
```

### Lambda Expressions

```text
LambdaExpression:
    LambdaIntroducer [AttributeSpecifierSeq] LambdaDeclarator CompoundStatement
    LambdaIntroducer < TemplateParameterList > [RequiresClause] [AttributeSpecifierSeq] LambdaDeclarator CompoundStatement

LambdaIntroducer:
    '[' [LambdaCapture] ']'

LambdaDeclarator:
    LambdaSpecifierSeq [NoexceptSpecifier] [AttributeSpecifierSeq] [TrailingReturnType] [FunctionContractSpecifierSeq]
    NoexceptSpecifier [AttributeSpecifierSeq] [TrailingReturnType] [FunctionContractSpecifierSeq]
    [TrailingReturnType] [FunctionContractSpecifierSeq]
    ( ParameterDeclarationClause ) [LambdaSpecifierSeq] [NoexceptSpecifier] [AttributeSpecifierSeq] [TrailingReturnType] [RequiresClause] [FunctionContractSpecifierSeq]

LambdaSpecifier: (one of)
    consteval  constexpr  mutable  static

LambdaSpecifierSeq:
    LambdaSpecifier {LambdaSpecifier}

LambdaCapture:
    CaptureDefault
    CaptureList
    CaptureDefault , CaptureList

CaptureDefault: (one of)
    &  =

CaptureList:
    Capture {, Capture}

Capture:
    SimpleCapture
    InitCapture

SimpleCapture:
    Identifier [...]
    & Identifier [...]
    this
    * this

InitCapture:
    [...] Identifier Initializer
    & [...] Identifier Initializer
```

### Fold, Requires, and Splice Expressions

```text
FoldExpression:
    ( CastExpression FoldOperator ... )
    ( ... FoldOperator CastExpression )
    ( CastExpression FoldOperator ... FoldOperator CastExpression )

FoldOperator: (one of)
    +    -    *    /    %    ^    &    |    <<   >>
    +=   -=   *=   /=   %=   ^=   &=   |=   <<=  >>=  =
    ==   !=   <    >    <=   >=   &&   ||   ,    .*   ->*

RequiresExpression:
    requires [RequirementParameterList] RequirementBody

RequirementParameterList:
    ( ParameterDeclarationClause )

RequirementBody:
    '{' RequirementSeq '}'

RequirementSeq:
    Requirement {Requirement}

Requirement:
    SimpleRequirement
    TypeRequirement
    CompoundRequirement
    NestedRequirement

SimpleRequirement:
    Expression ;

TypeRequirement:
    typename [NestedNameSpecifier] TypeName ;
    typename SpliceSpecifier ;
    typename SpliceSpecializationSpecifier ;

CompoundRequirement:
    '{' Expression '}' [NoexceptSpecifier] [ReturnTypeRequirement] ;

ReturnTypeRequirement:
    -> TypeConstraint

NestedRequirement:
    requires ConstraintExpression ;

SpliceExpression:
    SpliceSpecifier
    template SpliceSpecifier
    template SpliceSpecializationSpecifier
```

### Postfix and Unary Expressions

```text
PostfixExpression:
    PrimaryExpression
    PostfixExpression '[' [ExpressionList] ']'
    PostfixExpression ( [ExpressionList] )
    SimpleTypeSpecifier ( [ExpressionList] )
    TypenameSpecifier ( [ExpressionList] )
    SimpleTypeSpecifier BracedInitList
    TypenameSpecifier BracedInitList
    PostfixExpression . [template] IdExpression
    PostfixExpression . SpliceExpression
    PostfixExpression -> [template] IdExpression
    PostfixExpression -> SpliceExpression
    PostfixExpression ++
    PostfixExpression --
    dynamic_cast < TypeId > ( Expression )
    static_cast < TypeId > ( Expression )
    reinterpret_cast < TypeId > ( Expression )
    const_cast < TypeId > ( Expression )
    typeid ( Expression )
    typeid ( TypeId )

ExpressionList:
    InitializerList

UnaryExpression:
    PostfixExpression
    UnaryOperator CastExpression
    ++ CastExpression
    -- CastExpression
    AwaitExpression
    sizeof UnaryExpression
    sizeof ( NofunTypeId )
    sizeof ... ( Identifier )
    alignof ( NofunTypeId )
    NoexceptExpression
    NewExpression
    DeleteExpression
    ReflectExpression

UnaryOperator: (one of)
    *  &  +  -  !  ~

AwaitExpression:
    co_await CastExpression

NoexceptExpression:
    noexcept ( Expression )

ReflectExpression:
    ^^ ::
    ^^ ReflectionName
    ^^ TypeId
    ^^ IdExpression

ReflectionName:
    [NestedNameSpecifier] Identifier
    NestedNameSpecifier template Identifier
```

### New and Delete

```text
NewExpression:
    [::] new [NewPlacement] NewTypeId [NewInitializer]
    [::] new [NewPlacement] ( NofunTypeId ) [NewInitializer]

NewPlacement:
    ( ExpressionList )

NewTypeId:
    TypeSpecifierSeq [NewDeclarator]

NewDeclarator:
    PtrOperator [NewDeclarator]
    NoptrNewDeclarator

NoptrNewDeclarator:
    '[' [Expression] ']' [AttributeSpecifierSeq]
    NoptrNewDeclarator '[' ConstantExpression ']' [AttributeSpecifierSeq]

NewInitializer:
    ( [ExpressionList] )
    BracedInitList

DeleteExpression:
    [::] delete CastExpression
    [::] delete '[' ']' CastExpression
```

### Operator Precedence Chain

```text
CastExpression:
    UnaryExpression
    ( NofunTypeId ) CastExpression

PmExpression:
    CastExpression
    PmExpression .* CastExpression
    PmExpression ->* CastExpression

MultiplicativeExpression:
    PmExpression
    MultiplicativeExpression * PmExpression
    MultiplicativeExpression / PmExpression
    MultiplicativeExpression % PmExpression

AdditiveExpression:
    MultiplicativeExpression
    AdditiveExpression + MultiplicativeExpression
    AdditiveExpression - MultiplicativeExpression

ShiftExpression:
    AdditiveExpression
    ShiftExpression << AdditiveExpression
    ShiftExpression >> AdditiveExpression

CompareExpression:
    ShiftExpression
    CompareExpression <=> ShiftExpression

RelationalExpression:
    CompareExpression
    RelationalExpression < CompareExpression
    RelationalExpression > CompareExpression
    RelationalExpression <= CompareExpression
    RelationalExpression >= CompareExpression

EqualityExpression:
    RelationalExpression
    EqualityExpression == RelationalExpression
    EqualityExpression != RelationalExpression

AndExpression:
    EqualityExpression
    AndExpression & EqualityExpression

ExclusiveOrExpression:
    AndExpression
    ExclusiveOrExpression ^ AndExpression

InclusiveOrExpression:
    ExclusiveOrExpression
    InclusiveOrExpression | ExclusiveOrExpression

LogicalAndExpression:
    InclusiveOrExpression
    LogicalAndExpression && InclusiveOrExpression

LogicalOrExpression:
    LogicalAndExpression
    LogicalOrExpression || LogicalAndExpression

ConditionalExpression:
    LogicalOrExpression
    LogicalOrExpression ? Expression : AssignmentExpression

YieldExpression:
    co_yield AssignmentExpression
    co_yield BracedInitList

ThrowExpression:
    throw [AssignmentExpression]

AssignmentExpression:
    ConditionalExpression
    YieldExpression
    ThrowExpression
    LogicalOrExpression AssignmentOperator InitializerClause

AssignmentOperator: (one of)
    =    *=   /=   %=   +=   -=   >>=  <<=  &=   ^=   |=

Expression:
    AssignmentExpression {, AssignmentExpression}

ConstantExpression:
    ConditionalExpression
```

## Statements

```text
Statement:
    LabeledStatement
    [AttributeSpecifierSeq] ExpressionStatement
    [AttributeSpecifierSeq] CompoundStatement
    [AttributeSpecifierSeq] SelectionStatement
    [AttributeSpecifierSeq] IterationStatement
    [AttributeSpecifierSeq] ExpansionStatement
    [AttributeSpecifierSeq] JumpStatement
    [AttributeSpecifierSeq] AssertionStatement
    DeclarationStatement
    [AttributeSpecifierSeq] TryBlock

InitStatement:
    ExpressionStatement
    SimpleDeclaration
    AliasDeclaration

Condition:
    Expression
    ConditionDeclaration

ConditionDeclaration:
    [AttributeSpecifierSeq] DeclSpecifierSeq Declarator BraceOrEqualInitializer
    StructuredBindingDeclaration Initializer

ForRangeDeclaration:
    [AttributeSpecifierSeq] DeclSpecifierSeq Declarator
    StructuredBindingDeclaration

ForRangeInitializer:
    ExprOrBracedInitList

Label:
    [AttributeSpecifierSeq] Identifier :
    [AttributeSpecifierSeq] case ConstantExpression :
    [AttributeSpecifierSeq] default :

LabeledStatement:
    Label Statement

ExpressionStatement:
    [Expression] ;

CompoundStatement:
    '{' [StatementSeq] [LabelSeq] '}'

StatementSeq:
    Statement {Statement}

LabelSeq:
    Label {Label}

SelectionStatement:
    if [constexpr] ( [InitStatement] Condition ) Statement
    if [constexpr] ( [InitStatement] Condition ) Statement else Statement
    if [!] consteval CompoundStatement
    if [!] consteval CompoundStatement else Statement
    switch ( [InitStatement] Condition ) Statement

IterationStatement:
    while ( Condition ) Statement
    do Statement while ( Expression ) ;
    for ( InitStatement [Condition] ; [Expression] ) Statement
    for ( [InitStatement] ForRangeDeclaration : ForRangeInitializer ) Statement

ExpansionStatement:
    template for ( [InitStatement] ForRangeDeclaration : ExpansionInitializer ) CompoundStatement

ExpansionInitializer:
    Expression
    ExpansionInitList

ExpansionInitList:
    '{' ExpressionList [,] '}'
    '{' '}'

JumpStatement:
    break ;
    continue ;
    return [ExprOrBracedInitList] ;
    CoroutineReturnStatement
    goto Identifier ;

CoroutineReturnStatement:
    co_return [ExprOrBracedInitList] ;

AssertionStatement:
    contract_assert [AttributeSpecifierSeq] ( ConditionalExpression ) ;

DeclarationStatement:
    BlockDeclaration
```

## Declarations

```text
DeclarationSeq:
    Declaration {Declaration}

Declaration:
    NameDeclaration
    SpecialDeclaration

NameDeclaration:
    BlockDeclaration
    NodeclspecFunctionDeclaration
    FunctionDefinition
    FriendTypeDeclaration
    TemplateDeclaration
    DeductionGuide
    LinkageSpecification
    NamespaceDefinition
    EmptyDeclaration
    AttributeDeclaration
    ModuleImportDeclaration

SpecialDeclaration:
    ExplicitInstantiation
    ExplicitSpecialization
    ExportDeclaration

BlockDeclaration:
    SimpleDeclaration
    AsmDeclaration
    NamespaceAliasDefinition
    UsingDeclaration
    UsingEnumDeclaration
    UsingDirective
    StaticAssertDeclaration
    ConstevalBlockDeclaration
    AliasDeclaration
    OpaqueEnumDeclaration

NodeclspecFunctionDeclaration:
    [AttributeSpecifierSeq] Declarator ;

AliasDeclaration:
    using Identifier [AttributeSpecifierSeq] = DefiningTypeId ;

SimpleDeclaration:
    DeclSpecifierSeq [InitDeclaratorList] ;
    AttributeSpecifierSeq DeclSpecifierSeq InitDeclaratorList ;
    StructuredBindingDeclaration Initializer ;

StructuredBindingDeclaration:
    [AttributeSpecifierSeq] DeclSpecifierSeq [RefQualifier] '[' SbIdentifierList ']'

SbIdentifier:
    [...] Identifier [AttributeSpecifierSeq]

SbIdentifierList:
    SbIdentifier {, SbIdentifier}

StaticAssertDeclaration:
    static_assert ( ConstantExpression ) ;
    static_assert ( ConstantExpression , StaticAssertMessage ) ;

StaticAssertMessage:
    UnevaluatedString
    ConstantExpression

ConstevalBlockDeclaration:
    consteval CompoundStatement

EmptyDeclaration:
    ;

AttributeDeclaration:
    AttributeSpecifierSeq ;
```

### Specifiers

```text
DeclSpecifier:
    StorageClassSpecifier
    DefiningTypeSpecifier
    FunctionSpecifier
    friend
    typedef
    constexpr
    consteval
    constinit
    inline

DeclSpecifierSeq:
    DeclSpecifier [AttributeSpecifierSeq]
    DeclSpecifier DeclSpecifierSeq

StorageClassSpecifier: (one of)
    static  thread_local  extern  mutable

FunctionSpecifier:
    virtual
    ExplicitSpecifier

ExplicitSpecifier:
    explicit ( ConstantExpression )
    explicit

TypeSpecifier:
    SimpleTypeSpecifier
    ElaboratedTypeSpecifier
    TypenameSpecifier
    CvQualifier

TypeSpecifierSeq:
    TypeSpecifier [AttributeSpecifierSeq]
    TypeSpecifier TypeSpecifierSeq

DefiningTypeSpecifier:
    TypeSpecifier
    ClassSpecifier
    EnumSpecifier

DefiningTypeSpecifierSeq:
    DefiningTypeSpecifier [AttributeSpecifierSeq]
    DefiningTypeSpecifier DefiningTypeSpecifierSeq

SimpleTypeSpecifier:
    [NestedNameSpecifier] TypeName
    NestedNameSpecifier template SimpleTemplateId
    ComputedTypeSpecifier
    PlaceholderTypeSpecifier
    [NestedNameSpecifier] SimpleTemplateName
    PackIndexTemplateName
    char
    char8_t
    char16_t
    char32_t
    wchar_t
    bool
    short
    int
    long
    signed
    unsigned
    float
    double
    void

TypeName:
    ClassName
    EnumName
    TypedefName

ComputedTypeSpecifier:
    DecltypeSpecifier
    PackIndexSpecifier
    SpliceTypeSpecifier

DecltypeSpecifier:
    decltype ( Expression )

PackIndexSpecifier:
    TypedefName ... '[' ConstantExpression ']'

SpliceTypeSpecifier:
    [typename] SpliceSpecifier
    [typename] SpliceSpecializationSpecifier

PlaceholderTypeSpecifier:
    [TypeConstraint] auto
    [TypeConstraint] decltype ( auto )

ElaboratedTypeSpecifier:
    ClassKey [AttributeSpecifierSeq] [NestedNameSpecifier] Identifier
    ClassKey SimpleTemplateId
    ClassKey NestedNameSpecifier [template] SimpleTemplateId
    enum [NestedNameSpecifier] Identifier
```

### Declarators

```text
InitDeclaratorList:
    InitDeclarator {, InitDeclarator}

InitDeclarator:
    Declarator Initializer
    Declarator [RequiresClause] [FunctionContractSpecifierSeq]

Declarator:
    PtrDeclarator
    NoptrDeclarator ParametersAndQualifiers TrailingReturnType

PtrDeclarator:
    NoptrDeclarator
    PtrOperator PtrDeclarator

NoptrDeclarator:
    DeclaratorId [AttributeSpecifierSeq]
    NoptrDeclarator ParametersAndQualifiers
    NoptrDeclarator '[' [ConstantExpression] ']' [AttributeSpecifierSeq]
    ( PtrDeclarator )

ParametersAndQualifiers:
    ( ParameterDeclarationClause ) [CvQualifierSeq] [RefQualifier] [NoexceptSpecifier] [AttributeSpecifierSeq]

TrailingReturnType:
    -> NofunTypeId

PtrOperator:
    * [AttributeSpecifierSeq] [CvQualifierSeq]
    & [AttributeSpecifierSeq]
    && [AttributeSpecifierSeq]
    NestedNameSpecifier * [AttributeSpecifierSeq] [CvQualifierSeq]

CvQualifierSeq:
    CvQualifier {CvQualifier}

CvQualifier: (one of)
    const  volatile

RefQualifier: (one of)
    &  &&

DeclaratorId:
    [...] IdExpression
```

### Type Names

```text
TypeId:
    TypeSpecifierSeq [AbstractDeclarator]

DefiningTypeId:
    DefiningTypeSpecifierSeq [AbstractDeclarator]

NofunTypeId:
    TypeSpecifierSeq [NofunDeclarator]

NofunDeclarator:
    PtrNofunDeclarator
    NoptrNofunDeclarator ParametersAndQualifiers TrailingReturnType

PtrNofunDeclarator:
    NoptrNofunDeclarator
    PtrOperator [PtrNofunDeclarator]

NoptrNofunDeclarator:
    NoptrNofunDeclarator ParametersAndQualifiers
    [NoptrNofunDeclarator] '[' [ConstantExpression] ']' [AttributeSpecifierSeq]
    ( PtrNofunDeclarator )

AbstractDeclarator:
    PtrAbstractDeclarator
    [NoptrAbstractDeclarator] ParametersAndQualifiers TrailingReturnType
    AbstractPackDeclarator

PtrAbstractDeclarator:
    NoptrAbstractDeclarator
    PtrOperator [PtrAbstractDeclarator]

NoptrAbstractDeclarator:
    [NoptrAbstractDeclarator] ParametersAndQualifiers
    [NoptrAbstractDeclarator] '[' [ConstantExpression] ']' [AttributeSpecifierSeq]
    ( PtrAbstractDeclarator )

AbstractPackDeclarator:
    NoptrAbstractPackDeclarator
    PtrOperator AbstractPackDeclarator

NoptrAbstractPackDeclarator:
    NoptrAbstractPackDeclarator ParametersAndQualifiers
    ...
```

### Function Parameters and Contracts

```text
ParameterDeclarationClause:
    ...
    [ParameterDeclarationList]
    ParameterDeclarationList , ...
    ParameterDeclarationList ...

ParameterDeclarationList:
    ParameterDeclaration {, ParameterDeclaration}

ParameterDeclaration:
    [AttributeSpecifierSeq] [this] DeclSpecifierSeq Declarator
    [AttributeSpecifierSeq] DeclSpecifierSeq Declarator = InitializerClause
    [AttributeSpecifierSeq] [this] DeclSpecifierSeq [AbstractDeclarator]
    [AttributeSpecifierSeq] DeclSpecifierSeq [AbstractDeclarator] = InitializerClause

FunctionContractSpecifierSeq:
    FunctionContractSpecifier {FunctionContractSpecifier}

FunctionContractSpecifier:
    PreconditionSpecifier
    PostconditionSpecifier

PreconditionSpecifier:
    pre [AttributeSpecifierSeq] ( ConditionalExpression )

PostconditionSpecifier:
    post [AttributeSpecifierSeq] ( [ResultNameIntroducer] ConditionalExpression )

ResultNameIntroducer:
    AttributedIdentifier :

AttributedIdentifier:
    Identifier [AttributeSpecifierSeq]
```

### Initializers

```text
Initializer:
    BraceOrEqualInitializer
    ( ExpressionList )

BraceOrEqualInitializer:
    = InitializerClause
    BracedInitList

InitializerClause:
    AssignmentExpression
    BracedInitList

BracedInitList:
    '{' InitializerList [,] '}'
    '{' DesignatedInitializerList [,] '}'
    '{' '}'

InitializerList:
    InitializerClause [...] {, InitializerClause [...]}

DesignatedInitializerList:
    DesignatedOnlyInitializerList
    InitializerList , DesignatedOnlyInitializerList

DesignatedOnlyInitializerList:
    DesignatedInitializerClause {, DesignatedInitializerClause}

DesignatedInitializerClause:
    Designator BraceOrEqualInitializer

Designator:
    . Identifier

ExprOrBracedInitList:
    Expression
    BracedInitList
```

### Function Definitions

```text
FunctionDefinition:
    [AttributeSpecifierSeq] [DeclSpecifierSeq] Declarator [VirtSpecifierSeq] [FunctionContractSpecifierSeq] FunctionBody
    [AttributeSpecifierSeq] [DeclSpecifierSeq] Declarator RequiresClause [FunctionContractSpecifierSeq] FunctionBody

FunctionBody:
    [CtorInitializer] CompoundStatement
    FunctionTryBlock
    = default ;
    DeletedFunctionBody

DeletedFunctionBody:
    = delete ;
    = delete ( UnevaluatedString ) ;
```

### Enumerations

```text
EnumSpecifier:
    EnumHead '{' [EnumeratorList] '}'
    EnumHead '{' EnumeratorList , '}'

EnumHead:
    EnumKey [AttributeSpecifierSeq] [EnumHeadName] [EnumBase]

EnumHeadName:
    [NestedNameSpecifier] Identifier

OpaqueEnumDeclaration:
    EnumKey [AttributeSpecifierSeq] EnumHeadName [EnumBase] ;

EnumKey:
    enum
    enum class
    enum struct

EnumBase:
    : TypeSpecifierSeq

EnumeratorList:
    EnumeratorDefinition {, EnumeratorDefinition}

EnumeratorDefinition:
    Enumerator
    Enumerator = ConstantExpression

Enumerator:
    Identifier [AttributeSpecifierSeq]

UsingEnumDeclaration:
    using enum UsingEnumDeclarator ;

UsingEnumDeclarator:
    [NestedNameSpecifier] Identifier
    [NestedNameSpecifier] SimpleTemplateId
    SpliceTypeSpecifier
```

### Namespaces

```text
NamespaceDefinition:
    NamedNamespaceDefinition
    UnnamedNamespaceDefinition
    NestedNamespaceDefinition

NamedNamespaceDefinition:
    [inline] namespace [AttributeSpecifierSeq] Identifier '{' NamespaceBody '}'

UnnamedNamespaceDefinition:
    [inline] namespace [AttributeSpecifierSeq] '{' NamespaceBody '}'

NestedNamespaceDefinition:
    namespace EnclosingNamespaceSpecifier :: [inline] Identifier '{' NamespaceBody '}'

EnclosingNamespaceSpecifier:
    Identifier
    EnclosingNamespaceSpecifier :: [inline] Identifier

NamespaceBody:
    [DeclarationSeq]

NamespaceAliasDefinition:
    namespace Identifier = QualifiedNamespaceSpecifier ;
    namespace Identifier = SpliceSpecifier ;

QualifiedNamespaceSpecifier:
    [NestedNameSpecifier] NamespaceName

UsingDirective:
    [AttributeSpecifierSeq] using namespace [NestedNameSpecifier] NamespaceName ;
    [AttributeSpecifierSeq] using namespace SpliceSpecifier ;

UsingDeclaration:
    using UsingDeclaratorList ;

UsingDeclaratorList:
    UsingDeclarator [...] {, UsingDeclarator [...]}

UsingDeclarator:
    [typename] NestedNameSpecifier UnqualifiedId
```

### Assembly and Linkage

```text
AsmDeclaration:
    [AttributeSpecifierSeq] asm ( BalancedTokenSeq ) ;

LinkageSpecification:
    extern UnevaluatedString '{' [DeclarationSeq] '}'
    extern UnevaluatedString NameDeclaration
```

### Attributes and Annotations

```text
AttributeSpecifierSeq:
    AttributeSpecifier {AttributeSpecifier}

AttributeSpecifier:
    '[' '[' [AttributeUsingPrefix] AttributeList ']' ']'
    '[' '[' AnnotationList ']' ']'
    AlignmentSpecifier

AlignmentSpecifier:
    alignas ( NofunTypeId [...] )
    alignas ( ConstantExpression [...] )

AttributeUsingPrefix:
    using AttributeNamespace :

AttributeList:
    [Attribute [...]] {, [Attribute [...]]}

AnnotationList:
    Annotation [...] {, Annotation [...]}

Attribute:
    AttributeToken [AttributeArgumentClause]

Annotation:
    = ConstantExpression

AttributeToken:
    Identifier
    AttributeScopedToken

AttributeScopedToken:
    AttributeNamespace :: Identifier

AttributeNamespace:
    Identifier

AttributeArgumentClause:
    ( [BalancedTokenSeq] )

BalancedTokenSeq:
    BalancedToken {BalancedToken}

BalancedToken:
    ( [BalancedTokenSeq] )
    '[' [BalancedTokenSeq] ']'
    '{' [BalancedTokenSeq] '}'
    '[:' [BalancedTokenSeq] ':]'
    (any Token other than a parenthesis, bracket, brace, or splice delimiter)
```

> **On `AttributeList`.** The standard writes this as four alternatives, which never pairs an absent attribute with an ellipsis. The condensed form above reads as though it might; the accepted language is the same either way. See the note under **Notation**.

## Modules

```text
ModuleDeclaration:
    [ExportKeyword] ModuleKeyword ModuleName [ModulePartition] [AttributeSpecifierSeq] ;

ModuleName:
    [ModuleNameQualifier] Identifier

ModulePartition:
    : [ModuleNameQualifier] Identifier

ModuleNameQualifier:
    Identifier . {Identifier .}

ExportDeclaration:
    export NameDeclaration
    export '{' [DeclarationSeq] '}'
    ExportKeyword ModuleImportDeclaration

ModuleImportDeclaration:
    ImportKeyword ModuleName [AttributeSpecifierSeq] ;
    ImportKeyword ModulePartition [AttributeSpecifierSeq] ;
    ImportKeyword HeaderName [AttributeSpecifierSeq] ;

GlobalModuleFragment:
    ModuleKeyword ; [DeclarationSeq]

PrivateModuleFragment:
    ModuleKeyword : private ; [DeclarationSeq]
```

## Classes

```text
ClassName:
    Identifier
    SimpleTemplateId

ClassSpecifier:
    ClassHead '{' [MemberSpecification] '}'

ClassHead:
    ClassKey [AttributeSpecifierSeq] ClassHeadName [ClassPropertySpecifierSeq] [BaseClause]
    ClassKey [AttributeSpecifierSeq] [BaseClause]

ClassHeadName:
    [NestedNameSpecifier] ClassName

ClassPropertySpecifierSeq:
    ClassPropertySpecifier {ClassPropertySpecifier}

ClassPropertySpecifier:
    final

ClassKey: (one of)
    class  struct  union

MemberSpecification:
    MemberDeclaration [MemberSpecification]
    AccessSpecifier : [MemberSpecification]

MemberDeclaration:
    [AttributeSpecifierSeq] [DeclSpecifierSeq] [MemberDeclaratorList] ;
    FunctionDefinition
    FriendTypeDeclaration
    UsingDeclaration
    UsingEnumDeclaration
    StaticAssertDeclaration
    ConstevalBlockDeclaration
    TemplateDeclaration
    ExplicitSpecialization
    DeductionGuide
    AliasDeclaration
    OpaqueEnumDeclaration
    EmptyDeclaration

MemberDeclaratorList:
    MemberDeclarator {, MemberDeclarator}

MemberDeclarator:
    Declarator [VirtSpecifierSeq] [FunctionContractSpecifierSeq] [PureSpecifier]
    Declarator RequiresClause [FunctionContractSpecifierSeq]
    Declarator BraceOrEqualInitializer
    [Identifier] [AttributeSpecifierSeq] : ConstantExpression [BraceOrEqualInitializer]

VirtSpecifierSeq:
    VirtSpecifier {VirtSpecifier}

VirtSpecifier: (one of)
    override  final

PureSpecifier:
    = 0

FriendTypeDeclaration:
    friend FriendTypeSpecifierList ;

FriendTypeSpecifierList:
    FriendTypeSpecifier [...] {, FriendTypeSpecifier [...]}

FriendTypeSpecifier:
    SimpleTypeSpecifier
    ElaboratedTypeSpecifier
    TypenameSpecifier

ConversionFunctionId:
    operator ConversionTypeId

ConversionTypeId:
    TypeSpecifierSeq [ConversionDeclarator]

ConversionDeclarator:
    PtrOperator [ConversionDeclarator]
```

### Derived Classes

```text
BaseClause:
    : BaseSpecifierList

BaseSpecifierList:
    BaseSpecifier [...] {, BaseSpecifier [...]}

BaseSpecifier:
    [AttributeSpecifierSeq] ClassOrDecltype
    [AttributeSpecifierSeq] virtual [AccessSpecifier] ClassOrDecltype
    [AttributeSpecifierSeq] AccessSpecifier [virtual] ClassOrDecltype

ClassOrDecltype:
    [NestedNameSpecifier] TypeName
    NestedNameSpecifier template SimpleTemplateId
    ComputedTypeSpecifier

AccessSpecifier: (one of)
    private  protected  public
```

### Constructor Initializers

```text
CtorInitializer:
    : MemInitializerList

MemInitializerList:
    MemInitializer [...] {, MemInitializer [...]}

MemInitializer:
    MemInitializerId ( [ExpressionList] )
    MemInitializerId BracedInitList

MemInitializerId:
    ClassOrDecltype
    Identifier
```

## Overloading

```text
OperatorFunctionId:
    operator Operator

Operator: (one of)
    new  delete  new[]  delete[]  co_await  (  )  [  ]  ->  ->*
    ~    !       +      -         *         /   %   ^   &   |
    =    +=      -=     *=        /=        %=  ^=  &=  |=
    ==   !=      <      >         <=        >=  <=>  &&  ||
    <<   >>      <<=    >>=       ++        --  ,

LiteralOperatorId:
    operator UnevaluatedString Identifier
    operator UserDefinedStringLiteral
```

## Templates

```text
TemplateDeclaration:
    TemplateHead Declaration
    TemplateHead ConceptDefinition

TemplateHead:
    template < TemplateParameterList > [RequiresClause]

TemplateParameterList:
    TemplateParameter {, TemplateParameter}

RequiresClause:
    requires ConstraintLogicalOrExpression

ConstraintLogicalOrExpression:
    ConstraintLogicalAndExpression
    ConstraintLogicalOrExpression || ConstraintLogicalAndExpression

ConstraintLogicalAndExpression:
    PrimaryExpression
    ConstraintLogicalAndExpression && PrimaryExpression

TemplateParameter:
    TypeParameter
    ParameterDeclaration
    TypeTtParameter
    VariableTtParameter
    ConceptTtParameter

TypeParameter:
    TypeParameterKey [...] [Identifier]
    TypeParameterKey [Identifier] = TypeId
    TypeConstraint [...] [Identifier]
    TypeConstraint [Identifier] = TypeId

TypeParameterKey: (one of)
    class  typename

TypeConstraint:
    [NestedNameSpecifier] ConceptName
    [NestedNameSpecifier] ConceptName < [TemplateArgumentList] >

TypeTtParameter:
    TemplateHead TypeParameterKey [...] [Identifier]
    TemplateHead TypeParameterKey [Identifier] TypeTtParameterDefault

TypeTtParameterDefault:
    = [NestedNameSpecifier] TemplateName
    = NestedNameSpecifier template TemplateName

VariableTtParameter:
    TemplateHead auto [...] [Identifier]
    TemplateHead auto [Identifier] = [NestedNameSpecifier] TemplateName

ConceptTtParameter:
    template < TemplateParameterList > concept [...] [Identifier]
    template < TemplateParameterList > concept [Identifier] = [NestedNameSpecifier] TemplateName
```

### Template Names and Arguments

```text
SimpleTemplateId:
    TemplateName < [TemplateArgumentList] >

TemplateId:
    SimpleTemplateId
    OperatorFunctionId < [TemplateArgumentList] >
    LiteralOperatorId < [TemplateArgumentList] >

TemplateName:
    SimpleTemplateName
    PackIndexTemplateName

PackIndexTemplateName:
    SimpleTemplateName ... '[' ConstantExpression ']'

SimpleTemplateName:
    Identifier

TemplateArgumentList:
    TemplateArgument [...] {, TemplateArgument [...]}

TemplateArgument:
    TemplateArgumentName
    ConstantExpression
    TypeId
    BracedInitList

TemplateArgumentName:
    [NestedNameSpecifier] Identifier
    NestedNameSpecifier template Identifier

ConstraintExpression:
    LogicalOrExpression
```

### Deduction Guides, Concepts, and Instantiation

```text
DeductionGuide:
    [ExplicitSpecifier] SimpleTemplateName ( ParameterDeclarationClause ) -> SimpleTemplateId [RequiresClause] ;

ConceptDefinition:
    concept ConceptName [AttributeSpecifierSeq] = ConstraintExpression ;

ConceptName:
    Identifier

TypenameSpecifier:
    typename NestedNameSpecifier Identifier
    typename NestedNameSpecifier [template] SimpleTemplateId

ExplicitInstantiation:
    [extern] template Declaration

ExplicitSpecialization:
    template < > Declaration
```

## Exception Handling

```text
TryBlock:
    try CompoundStatement HandlerSeq

FunctionTryBlock:
    try [CtorInitializer] CompoundStatement HandlerSeq

HandlerSeq:
    Handler {Handler}

Handler:
    catch ( ExceptionDeclaration ) CompoundStatement

ExceptionDeclaration:
    [AttributeSpecifierSeq] TypeSpecifierSeq Declarator
    [AttributeSpecifierSeq] TypeSpecifierSeq [AbstractDeclarator]
    ...

NoexceptSpecifier:
    noexcept ( ConstantExpression )
    noexcept
```

## Preprocessing Directives

```text
PreprocessingFile:
    [Group]
    ModuleFile

ModuleFile:
    [LineDirectives] [PpGlobalModuleFragment] PpModule [Group] [PpPrivateModuleFragment]

PpGlobalModuleFragment:
    module ; NewLine [Group]

PpPrivateModuleFragment:
    module : private ; NewLine [Group]

Group:
    GroupPart {GroupPart}

GroupPart:
    ControlLine
    IfSection
    TextLine
    # ConditionallySupportedDirective

ControlLine:
    # include PpTokens NewLine
    PpImport
    # embed PpTokens NewLine
    # define Identifier ReplacementList NewLine
    # define Identifier Lparen [IdentifierList] ) ReplacementList NewLine
    # define Identifier Lparen ... ) ReplacementList NewLine
    # define Identifier Lparen IdentifierList , ... ) ReplacementList NewLine
    # undef Identifier NewLine
    LineDirective
    # error [PpTokens] NewLine
    # warning [PpTokens] NewLine
    # pragma [PpTokens] NewLine
    # NewLine

LineDirectives:
    LineDirective {LineDirective}

LineDirective:
    # line PpTokens NewLine
```

### Conditional Inclusion

```text
IfSection:
    IfGroup [ElifGroups] [ElseGroup] EndifLine

IfGroup:
    # if ConstantExpression NewLine [Group]
    # ifdef Identifier NewLine [Group]
    # ifndef Identifier NewLine [Group]

ElifGroups:
    ElifGroup {ElifGroup}

ElifGroup:
    # elif ConstantExpression NewLine [Group]
    # elifdef Identifier NewLine [Group]
    # elifndef Identifier NewLine [Group]

ElseGroup:
    # else NewLine [Group]

EndifLine:
    # endif NewLine

DefinedMacroExpression:
    defined Identifier
    defined ( Identifier )

HPreprocessingToken:
    (any PreprocessingToken other than U+003E greater-than sign)

HPpTokens:
    HPreprocessingToken {HPreprocessingToken}

HeaderNameTokens:
    PlainStringLiteral
    < HPpTokens >

HasIncludeExpression:
    __has_include ( HeaderName )
    __has_include ( HeaderNameTokens )

HasEmbedExpression:
    __has_embed ( HeaderName [PpBalancedTokenSeq] )
    __has_embed ( HeaderNameTokens [PpBalancedTokenSeq] )

HasAttributeExpression:
    __has_cpp_attribute ( PpTokens )
```

### Directive Bodies and Token Sequences

```text
TextLine:
    [PpTokens] NewLine

ConditionallySupportedDirective:
    PpTokens NewLine

Lparen:
    (a U+0028 left parenthesis not immediately preceded by whitespace)

IdentifierList:
    Identifier {, Identifier}

ReplacementList:
    [PpTokens]

PpTokens:
    PreprocessingToken {PreprocessingToken}

NewLine:
    (the new-line character)

VaOptReplacement:
    __VA_OPT__ ( [PpTokens] )
```

### Resource Inclusion (`#embed`)

```text
EmbedParameterSeq:
    EmbedParameter {EmbedParameter}

EmbedParameter:
    EmbedStandardParameter
    EmbedPrefixedParameter

EmbedStandardParameter:
    limit ( PpBalancedTokenSeq )
    offset ( PpBalancedTokenSeq )
    prefix ( [PpBalancedTokenSeq] )
    suffix ( [PpBalancedTokenSeq] )
    if_empty ( [PpBalancedTokenSeq] )

EmbedPrefixedParameter:
    Identifier :: Identifier
    Identifier :: Identifier ( [PpBalancedTokenSeq] )

PpBalancedTokenSeq:
    PpBalancedToken {PpBalancedToken}

PpBalancedToken:
    ( [PpBalancedTokenSeq] )
    '[' [PpBalancedTokenSeq] ']'
    '{' [PpBalancedTokenSeq] '}'
    (any PreprocessingToken other than a parenthesis, bracket, or brace)
```

### Module and Header Unit Directives

```text
PpModule:
    [export] module [PpTokens] NewLine

PpImport:
    [export] import HeaderName [PpTokens] ; NewLine
    [export] import HeaderNameTokens [PpTokens] ; NewLine
    [export] import PpTokens ; NewLine
```