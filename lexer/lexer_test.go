package lexer

import (
	"fmt"
	"testing"
)

// --- Assertion helpers (per-extension pattern, no cross-extension deps) ---

func assertNoErrors(t testing.TB, errs []LexError) {
	t.Helper()
	if len(errs) > 0 {
		t.Fatalf("unexpected lex errors: %v", errs)
	}
}

func assertHasErrors(t testing.TB, errs []LexError) {
	t.Helper()
	if len(errs) == 0 {
		t.Fatal("expected lex errors, got none")
	}
}

func assertTokenTypes(t testing.TB, tokens []Token, want []TokenType) {
	t.Helper()
	got := make([]TokenType, len(tokens))
	for i, tok := range tokens {
		got[i] = tok.Type
	}
	if len(got) != len(want) {
		t.Fatalf("token count: got %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token[%d]: got %v, want %v", i, got[i], want[i])
		}
	}
}

func assertTokenValues(t testing.TB, tokens []Token, want []string) {
	t.Helper()
	got := make([]string, len(tokens))
	for i, tok := range tokens {
		got[i] = tok.Val
	}
	if len(got) != len(want) {
		t.Fatalf("token count: got %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token[%d] val: got %q, want %q", i, got[i], want[i])
		}
	}
}

// findTokens filters to non-EOF, non-NEWLINE tokens for easier assertions.
func findTokens(tokens []Token, tt TokenType) []Token {
	var result []Token
	for _, tok := range tokens {
		if tok.Type == tt {
			result = append(result, tok)
		}
	}
	return result
}

// --- Tests ---

func TestTokenize_EmptySource(t *testing.T) {
	tokens, errs := Tokenize("")
	assertNoErrors(t, errs)
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token (EOF), got %d", len(tokens))
	}
	if tokens[0].Type != TokenEOF {
		t.Errorf("expected EOF, got %v", tokens[0].Type)
	}
}

func TestTokenize_WhitespaceOnly(t *testing.T) {
	tokens, errs := Tokenize("   \t  ")
	assertNoErrors(t, errs)
	// Should just have EOF (spaces consumed, no newline)
	if tokens[len(tokens)-1].Type != TokenEOF {
		t.Errorf("expected final EOF")
	}
}

func TestTokenize_IntLiterals(t *testing.T) {
	tests := []struct {
		src  string
		want string
	}{
		{"0", "0"},
		{"42", "42"},
		{"123456", "123456"},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			tokens, errs := Tokenize(tc.src)
			assertNoErrors(t, errs)
			ints := findTokens(tokens, TokenInt)
			if len(ints) != 1 {
				t.Fatalf("expected 1 INT token, got %d", len(ints))
			}
			if ints[0].Val != tc.want {
				t.Errorf("got %q, want %q", ints[0].Val, tc.want)
			}
		})
	}
}

func TestTokenize_FloatLiterals(t *testing.T) {
	tests := []struct {
		src  string
		want string
	}{
		{"3.14", "3.14"},
		{"0.5", "0.5"},
		{"100.0", "100.0"},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			tokens, errs := Tokenize(tc.src)
			assertNoErrors(t, errs)
			floats := findTokens(tokens, TokenFloat)
			if len(floats) != 1 {
				t.Fatalf("expected 1 FLOAT token, got %d", len(floats))
			}
			if floats[0].Val != tc.want {
				t.Errorf("got %q, want %q", floats[0].Val, tc.want)
			}
		})
	}
}

func TestTokenize_IntNotFloatWithDotDot(t *testing.T) {
	// "1..10" should be INT(1) DOTDOT INT(10), not FLOAT
	tokens, errs := Tokenize("1..10")
	assertNoErrors(t, errs)
	assertTokenTypes(t, tokens, []TokenType{TokenInt, TokenDotDot, TokenInt, TokenEOF})
	assertTokenValues(t, tokens, []string{"1", "..", "10", ""})
}

func TestTokenize_StringLiterals(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"empty", `""`, ""},
		{"simple", `"hello"`, "hello"},
		{"with spaces", `"hello world"`, "hello world"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, errs := Tokenize(tc.src)
			assertNoErrors(t, errs)
			strs := findTokens(tokens, TokenString)
			if len(strs) != 1 {
				t.Fatalf("expected 1 STRING token, got %d (all: %v)", len(strs), tokens)
			}
			if strs[0].Val != tc.want {
				t.Errorf("got %q, want %q", strs[0].Val, tc.want)
			}
		})
	}
}

func TestTokenize_BoolLiterals(t *testing.T) {
	tokens, errs := Tokenize("true false")
	assertNoErrors(t, errs)
	trues := findTokens(tokens, TokenTrue)
	falses := findTokens(tokens, TokenFalse)
	if len(trues) != 1 {
		t.Errorf("expected 1 true token, got %d", len(trues))
	}
	if len(falses) != 1 {
		t.Errorf("expected 1 false token, got %d", len(falses))
	}
}

func TestTokenize_NullLiteral(t *testing.T) {
	tokens, errs := Tokenize("null")
	assertNoErrors(t, errs)
	nulls := findTokens(tokens, TokenNull)
	if len(nulls) != 1 {
		t.Fatalf("expected 1 null token, got %d", len(nulls))
	}
}

func TestTokenize_AllKeywords(t *testing.T) {
	kws := []struct {
		word string
		tt   TokenType
	}{
		{"fn", TokenFn},
		{"let", TokenLet},
		{"return", TokenReturn},
		{"if", TokenIf},
		{"then", TokenThen},
		{"else", TokenElse},
		{"match", TokenMatch},
		{"when", TokenWhen},
		{"try", TokenTry},
		{"catch", TokenCatch},
		{"and", TokenAnd},
		{"or", TokenOr},
		{"not", TokenNot},
		{"query", TokenQuery},
	}
	for _, kw := range kws {
		t.Run(kw.word, func(t *testing.T) {
			tokens, errs := Tokenize(kw.word)
			assertNoErrors(t, errs)
			found := findTokens(tokens, kw.tt)
			if len(found) != 1 {
				t.Fatalf("expected 1 %v token for %q, got %d (all: %v)", kw.tt, kw.word, len(found), tokens)
			}
			if found[0].Val != kw.word {
				t.Errorf("got val %q, want %q", found[0].Val, kw.word)
			}
		})
	}
}

func TestTokenize_Identifiers(t *testing.T) {
	tests := []struct {
		src  string
		want string
	}{
		{"x", "x"},
		{"myVar", "myVar"},
		{"_private", "_private"},
		{"name_with_underscores", "name_with_underscores"},
		{"abc123", "abc123"},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			tokens, errs := Tokenize(tc.src)
			assertNoErrors(t, errs)
			idents := findTokens(tokens, TokenIdent)
			if len(idents) != 1 {
				t.Fatalf("expected 1 IDENT token, got %d", len(idents))
			}
			if idents[0].Val != tc.want {
				t.Errorf("got %q, want %q", idents[0].Val, tc.want)
			}
		})
	}
}

func TestTokenize_DollarParam(t *testing.T) {
	tokens, errs := Tokenize("$price")
	assertNoErrors(t, errs)
	idents := findTokens(tokens, TokenIdent)
	if len(idents) != 1 {
		t.Fatalf("expected 1 IDENT token, got %d", len(idents))
	}
	if idents[0].Val != "$price" {
		t.Errorf("got %q, want %q", idents[0].Val, "$price")
	}
}

func TestTokenize_DollarNoIdent(t *testing.T) {
	// $ followed by non-ident char should produce an error
	_, errs := Tokenize("$ ")
	assertHasErrors(t, errs)
}

func TestTokenize_AllOperators(t *testing.T) {
	tests := []struct {
		src string
		tt  TokenType
	}{
		{"+", TokenPlus},
		{"-", TokenMinus},
		{"*", TokenStar},
		{"/", TokenSlash},
		{"%", TokenPercent},
		{"==", TokenEq},
		{"!=", TokenNe},
		{">", TokenGt},
		{"<", TokenLt},
		{">=", TokenGe},
		{"<=", TokenLe},
		{"&&", TokenAmpAmp},
		{"||", TokenPipePipe},
		{"!", TokenBang},
		{"|", TokenPipe},
		{"=>", TokenArrow},
		{"->", TokenThinArrow},
		{"..", TokenDotDot},
		{"??", TokenQQ},
		{"?.", TokenQDot},
		{"++", TokenPlusPlus},
		{".", TokenDot},
		{":", TokenColon},
		{"::", TokenColonColon},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			tokens, errs := Tokenize(tc.src)
			assertNoErrors(t, errs)
			found := findTokens(tokens, tc.tt)
			if len(found) != 1 {
				t.Fatalf("expected 1 %v token for %q, got %d (all: %v)", tc.tt, tc.src, len(found), tokens)
			}
		})
	}
}

func TestTokenize_AllDelimiters(t *testing.T) {
	tests := []struct {
		src string
		tt  TokenType
	}{
		{"(", TokenLParen},
		{")", TokenRParen},
		{"[", TokenLBracket},
		{"]", TokenRBracket},
		{"{", TokenLBrace},
		{"}", TokenRBrace},
		{",", TokenComma},
		{"...", TokenEllipsis},
		{"=", TokenAssign},
	}
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			tokens, errs := Tokenize(tc.src)
			assertNoErrors(t, errs)
			found := findTokens(tokens, tc.tt)
			if len(found) != 1 {
				t.Fatalf("expected 1 %v token for %q, got %d (all: %v)", tc.tt, tc.src, len(found), tokens)
			}
		})
	}
}

func TestTokenize_EscapeSequences(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"newline", `"hello\nworld"`, "hello\nworld"},
		{"tab", `"hello\tworld"`, "hello\tworld"},
		{"carriage return", `"hello\rworld"`, "hello\rworld"},
		{"backslash", `"hello\\world"`, "hello\\world"},
		{"quote", `"hello\"world"`, "hello\"world"},
		{"escaped brace", `"hello\{world"`, "hello{world"},
		{"unknown escape", `"hello\xworld"`, "hello\\xworld"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, errs := Tokenize(tc.src)
			assertNoErrors(t, errs)
			strs := findTokens(tokens, TokenString)
			if len(strs) != 1 {
				t.Fatalf("expected 1 STRING token, got %d", len(strs))
			}
			if strs[0].Val != tc.want {
				t.Errorf("got %q, want %q", strs[0].Val, tc.want)
			}
		})
	}
}

func TestTokenize_StringInterpolation(t *testing.T) {
	// "hello {name} world"
	tokens, errs := Tokenize(`"hello {name} world"`)
	assertNoErrors(t, errs)

	// Expected sequence: STRING_START("hello ") INTERP_START("{") IDENT("name") INTERP_END("}") STRING_END(" world") EOF
	expectedTypes := []TokenType{
		TokenStringStart, TokenInterpStart, TokenIdent, TokenInterpEnd, TokenStringEnd, TokenEOF,
	}
	assertTokenTypes(t, tokens, expectedTypes)

	// Verify the string parts
	if tokens[0].Val != "hello " {
		t.Errorf("StringStart val: got %q, want %q", tokens[0].Val, "hello ")
	}
	if tokens[2].Val != "name" {
		t.Errorf("Ident val: got %q, want %q", tokens[2].Val, "name")
	}
	if tokens[4].Val != " world" {
		t.Errorf("StringEnd val: got %q, want %q", tokens[4].Val, " world")
	}
}

func TestTokenize_StringInterpolation_MultipleExprs(t *testing.T) {
	// "{a} and {b}"
	tokens, errs := Tokenize(`"{a} and {b}"`)
	assertNoErrors(t, errs)

	// STRING_START("") INTERP_START IDENT(a) INTERP_END STRING_PART(" and ") INTERP_START IDENT(b) INTERP_END STRING_END("") EOF
	expectedTypes := []TokenType{
		TokenStringStart, TokenInterpStart, TokenIdent, TokenInterpEnd,
		TokenStringPart, TokenInterpStart, TokenIdent, TokenInterpEnd,
		TokenStringEnd, TokenEOF,
	}
	assertTokenTypes(t, tokens, expectedTypes)
}

func TestTokenize_LineComment(t *testing.T) {
	tokens, errs := Tokenize("x -- this is a comment\ny")
	assertNoErrors(t, errs)

	idents := findTokens(tokens, TokenIdent)
	if len(idents) != 2 {
		t.Fatalf("expected 2 IDENT tokens, got %d", len(idents))
	}
	if idents[0].Val != "x" || idents[1].Val != "y" {
		t.Errorf("got idents %q, %q, want x, y", idents[0].Val, idents[1].Val)
	}
}

func TestTokenize_BlockComment(t *testing.T) {
	tokens, errs := Tokenize("x {- block comment -} y")
	assertNoErrors(t, errs)

	idents := findTokens(tokens, TokenIdent)
	if len(idents) != 2 {
		t.Fatalf("expected 2 IDENT tokens, got %d", len(idents))
	}
	if idents[0].Val != "x" || idents[1].Val != "y" {
		t.Errorf("got idents %q, %q, want x, y", idents[0].Val, idents[1].Val)
	}
}

func TestTokenize_NestedBlockComment(t *testing.T) {
	tokens, errs := Tokenize("x {- outer {- inner -} outer -} y")
	assertNoErrors(t, errs)

	idents := findTokens(tokens, TokenIdent)
	if len(idents) != 2 {
		t.Fatalf("expected 2 IDENT tokens, got %d", len(idents))
	}
	if idents[0].Val != "x" || idents[1].Val != "y" {
		t.Errorf("got idents %q, %q, want x, y", idents[0].Val, idents[1].Val)
	}
}

func TestTokenize_UnterminatedBlockComment(t *testing.T) {
	_, errs := Tokenize("{- never closed")
	assertHasErrors(t, errs)
}

func TestTokenize_IndentDedent(t *testing.T) {
	src := "if true\n  x\n  y\nz"
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	// Expected: IF TRUE NEWLINE INDENT IDENT(x) NEWLINE IDENT(y) NEWLINE DEDENT IDENT(z) EOF
	indents := findTokens(tokens, TokenIndent)
	dedents := findTokens(tokens, TokenDedent)
	if len(indents) != 1 {
		t.Errorf("expected 1 INDENT, got %d", len(indents))
	}
	if len(dedents) != 1 {
		t.Errorf("expected 1 DEDENT, got %d", len(dedents))
	}
}

func TestTokenize_MultiLevelIndent(t *testing.T) {
	src := "a\n  b\n    c\nd"
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	indents := findTokens(tokens, TokenIndent)
	dedents := findTokens(tokens, TokenDedent)
	if len(indents) != 2 {
		t.Errorf("expected 2 INDENTs, got %d", len(indents))
	}
	if len(dedents) != 2 {
		t.Errorf("expected 2 DEDENTs, got %d", len(dedents))
	}
}

func TestTokenize_DedentAtEOF(t *testing.T) {
	// Remaining indentation levels should be emitted as DEDENTs at EOF
	src := "a\n  b"
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	indents := findTokens(tokens, TokenIndent)
	dedents := findTokens(tokens, TokenDedent)
	if len(indents) != 1 {
		t.Errorf("expected 1 INDENT, got %d", len(indents))
	}
	if len(dedents) != 1 {
		t.Errorf("expected 1 DEDENT at EOF, got %d", len(dedents))
	}
}

func TestTokenize_TabAsIndentation(t *testing.T) {
	// A tab counts as 4 spaces
	src := "a\n\tb"
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	indents := findTokens(tokens, TokenIndent)
	if len(indents) != 1 {
		t.Errorf("expected 1 INDENT from tab, got %d", len(indents))
	}
}

func TestTokenize_BlankLineIgnored(t *testing.T) {
	// Blank lines should not emit spurious INDENT/DEDENT
	src := "a\n\nb"
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	indents := findTokens(tokens, TokenIndent)
	dedents := findTokens(tokens, TokenDedent)
	if len(indents) != 0 {
		t.Errorf("expected 0 INDENTs for blank line, got %d", len(indents))
	}
	if len(dedents) != 0 {
		t.Errorf("expected 0 DEDENTs for blank line, got %d", len(dedents))
	}
}

func TestTokenize_Newline(t *testing.T) {
	tokens, errs := Tokenize("a\nb")
	assertNoErrors(t, errs)

	newlines := findTokens(tokens, TokenNewline)
	if len(newlines) != 1 {
		t.Errorf("expected 1 NEWLINE, got %d", len(newlines))
	}
}

func TestTokenize_CarriageReturnNewline(t *testing.T) {
	tokens, errs := Tokenize("a\r\nb")
	assertNoErrors(t, errs)

	newlines := findTokens(tokens, TokenNewline)
	if len(newlines) != 1 {
		t.Errorf("expected 1 NEWLINE from \\r\\n, got %d", len(newlines))
	}
}

func TestTokenize_UnterminatedString(t *testing.T) {
	_, errs := Tokenize(`"hello`)
	assertHasErrors(t, errs)
}

func TestTokenize_UnterminatedStringNewline(t *testing.T) {
	// A newline inside a string should trigger an error
	_, errs := Tokenize("\"hello\nworld\"")
	assertHasErrors(t, errs)
}

func TestTokenize_UnexpectedCharacter(t *testing.T) {
	_, errs := Tokenize("@")
	assertHasErrors(t, errs)
}

func TestTokenize_SingleAmpersand(t *testing.T) {
	// Single & (not &&) is unexpected
	_, errs := Tokenize("&")
	assertHasErrors(t, errs)
}

func TestTokenize_SingleQuestionMark(t *testing.T) {
	// A lone ? is a valid TokenQuestion: it marks optional fields in record/type
	// definitions (e.g. `name?: type`). Only ?? and ?. are distinct operators.
	tokens, errs := Tokenize("?")
	assertNoErrors(t, errs)

	assertTokenTypes(t, tokens, []TokenType{TokenQuestion, TokenEOF})
}

func TestTokenize_ComplexExpression(t *testing.T) {
	src := "x + y * 2 - 3.14"
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	expectedTypes := []TokenType{
		TokenIdent, TokenPlus, TokenIdent, TokenStar, TokenInt, TokenMinus, TokenFloat, TokenEOF,
	}
	assertTokenTypes(t, tokens, expectedTypes)
}

func TestTokenize_FunctionDefinition(t *testing.T) {
	src := "fn add(a, b) => a + b"
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	expectedTypes := []TokenType{
		TokenFn, TokenIdent, TokenLParen, TokenIdent, TokenComma, TokenIdent, TokenRParen,
		TokenArrow, TokenIdent, TokenPlus, TokenIdent, TokenEOF,
	}
	assertTokenTypes(t, tokens, expectedTypes)
}

func TestTokenize_LetBinding(t *testing.T) {
	src := `let name = "hello"`
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	expectedTypes := []TokenType{
		TokenLet, TokenIdent, TokenAssign, TokenString, TokenEOF,
	}
	assertTokenTypes(t, tokens, expectedTypes)
}

func TestTokenize_MatchExpression(t *testing.T) {
	src := "match x\n  when 1 => a\n  when 2 => b"
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	matchTokens := findTokens(tokens, TokenMatch)
	whenTokens := findTokens(tokens, TokenWhen)
	if len(matchTokens) != 1 {
		t.Errorf("expected 1 MATCH token, got %d", len(matchTokens))
	}
	if len(whenTokens) != 2 {
		t.Errorf("expected 2 WHEN tokens, got %d", len(whenTokens))
	}
}

func TestTokenize_PipeAndPipePipe(t *testing.T) {
	// Single | is pipe, || is logical or
	tokens, errs := Tokenize("a | b || c")
	assertNoErrors(t, errs)

	pipes := findTokens(tokens, TokenPipe)
	pipePipes := findTokens(tokens, TokenPipePipe)
	if len(pipes) != 1 {
		t.Errorf("expected 1 PIPE, got %d", len(pipes))
	}
	if len(pipePipes) != 1 {
		t.Errorf("expected 1 PIPE_PIPE, got %d", len(pipePipes))
	}
}

func TestTokenize_NullCoalescing(t *testing.T) {
	tokens, errs := Tokenize("x ?? y")
	assertNoErrors(t, errs)

	assertTokenTypes(t, tokens, []TokenType{TokenIdent, TokenQQ, TokenIdent, TokenEOF})
}

func TestTokenize_OptionalChaining(t *testing.T) {
	tokens, errs := Tokenize("x?.y")
	assertNoErrors(t, errs)

	assertTokenTypes(t, tokens, []TokenType{TokenIdent, TokenQDot, TokenIdent, TokenEOF})
}

func TestTokenize_RangeOperator(t *testing.T) {
	tokens, errs := Tokenize("1..10")
	assertNoErrors(t, errs)

	assertTokenTypes(t, tokens, []TokenType{TokenInt, TokenDotDot, TokenInt, TokenEOF})
}

func TestTokenize_EllipsisOperator(t *testing.T) {
	tokens, errs := Tokenize("...args")
	assertNoErrors(t, errs)

	assertTokenTypes(t, tokens, []TokenType{TokenEllipsis, TokenIdent, TokenEOF})
}

func TestTokenize_ColonColon(t *testing.T) {
	tokens, errs := Tokenize("math::sqrt")
	assertNoErrors(t, errs)

	assertTokenTypes(t, tokens, []TokenType{TokenIdent, TokenColonColon, TokenIdent, TokenEOF})
}

func TestTokenize_TryCatch(t *testing.T) {
	src := "try x catch e => null"
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	expectedTypes := []TokenType{
		TokenTry, TokenIdent, TokenCatch, TokenIdent, TokenArrow, TokenNull, TokenEOF,
	}
	assertTokenTypes(t, tokens, expectedTypes)
}

func TestTokenize_ListAndObjectLiterals(t *testing.T) {
	src := `[1, 2, {a: 3}]`
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	expectedTypes := []TokenType{
		TokenLBracket, TokenInt, TokenComma, TokenInt, TokenComma,
		TokenLBrace, TokenIdent, TokenColon, TokenInt, TokenRBrace,
		TokenRBracket, TokenEOF,
	}
	assertTokenTypes(t, tokens, expectedTypes)
}

func TestTokenize_Position(t *testing.T) {
	tokens, errs := Tokenize("a b")
	assertNoErrors(t, errs)

	idents := findTokens(tokens, TokenIdent)
	if len(idents) != 2 {
		t.Fatalf("expected 2 idents, got %d", len(idents))
	}
	// First ident at col 1, second at col 3
	if idents[0].Pos.Line != 1 || idents[0].Pos.Column != 1 {
		t.Errorf("first ident pos: got (%d,%d), want (1,1)", idents[0].Pos.Line, idents[0].Pos.Column)
	}
	if idents[1].Pos.Line != 1 || idents[1].Pos.Column != 3 {
		t.Errorf("second ident pos: got (%d,%d), want (1,3)", idents[1].Pos.Line, idents[1].Pos.Column)
	}
}

func TestTokenize_PositionMultiLine(t *testing.T) {
	tokens, errs := Tokenize("a\nb")
	assertNoErrors(t, errs)

	idents := findTokens(tokens, TokenIdent)
	if len(idents) != 2 {
		t.Fatalf("expected 2 idents, got %d", len(idents))
	}
	if idents[0].Pos.Line != 1 {
		t.Errorf("first ident line: got %d, want 1", idents[0].Pos.Line)
	}
	if idents[1].Pos.Line != 2 {
		t.Errorf("second ident line: got %d, want 2", idents[1].Pos.Line)
	}
}

func TestTokenType_String(t *testing.T) {
	tests := []struct {
		tt   TokenType
		want string
	}{
		{TokenEOF, "EOF"},
		{TokenInt, "INT"},
		{TokenPlus, "+"},
		{TokenFn, "fn"},
		{TokenString, "STRING"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := tc.tt.String()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTokenType_String_Unknown(t *testing.T) {
	// Out-of-range token type should produce "Token(N)" format
	unknown := TokenType(9999)
	got := unknown.String()
	want := fmt.Sprintf("Token(%d)", 9999)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestToken_String(t *testing.T) {
	tok := Token{Type: TokenIdent, Val: "foo"}
	got := tok.String()
	want := `IDENT("foo")`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Token with empty val
	eof := Token{Type: TokenEOF, Val: ""}
	got = eof.String()
	want = "EOF"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLexError_Error(t *testing.T) {
	_, errs := Tokenize("@")
	if len(errs) == 0 {
		t.Fatal("expected error")
	}
	msg := errs[0].Error()
	if msg == "" {
		t.Error("error message should not be empty")
	}
	// Should contain line and column info
	if !containsStr(msg, "line") || !containsStr(msg, "col") {
		t.Errorf("error message should contain line/col info: %q", msg)
	}
}

func TestTokenize_CommentOnlyLineIgnoredForIndent(t *testing.T) {
	// A line with only a comment should not trigger indentation changes
	src := "a\n  -- comment\nb"
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	// The comment-only line should not produce INDENT/DEDENT
	idents := findTokens(tokens, TokenIdent)
	if len(idents) != 2 {
		t.Fatalf("expected 2 idents (a, b), got %d", len(idents))
	}
}

func TestTokenize_MultipleTokensSameLine(t *testing.T) {
	src := "1 + 2 * 3"
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	expectedTypes := []TokenType{
		TokenInt, TokenPlus, TokenInt, TokenStar, TokenInt, TokenEOF,
	}
	assertTokenTypes(t, tokens, expectedTypes)
}

func TestTokenize_IfThenElse(t *testing.T) {
	src := "if x then y else z"
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	expectedTypes := []TokenType{
		TokenIf, TokenIdent, TokenThen, TokenIdent, TokenElse, TokenIdent, TokenEOF,
	}
	assertTokenTypes(t, tokens, expectedTypes)
}

func TestTokenize_ComparisonOperators(t *testing.T) {
	src := "a == b != c > d < e >= f <= g"
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	expectedTypes := []TokenType{
		TokenIdent, TokenEq, TokenIdent, TokenNe, TokenIdent, TokenGt, TokenIdent,
		TokenLt, TokenIdent, TokenGe, TokenIdent, TokenLe, TokenIdent, TokenEOF,
	}
	assertTokenTypes(t, tokens, expectedTypes)
}

func TestTokenize_LambdaArrow(t *testing.T) {
	src := "x => x + 1"
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	assertTokenTypes(t, tokens, []TokenType{
		TokenIdent, TokenArrow, TokenIdent, TokenPlus, TokenInt, TokenEOF,
	})
}

func TestTokenize_ThinArrow(t *testing.T) {
	src := "a -> b"
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	assertTokenTypes(t, tokens, []TokenType{
		TokenIdent, TokenThinArrow, TokenIdent, TokenEOF,
	})
}

func TestTokenize_PlusPlusConcat(t *testing.T) {
	src := "a ++ b"
	tokens, errs := Tokenize(src)
	assertNoErrors(t, errs)

	assertTokenTypes(t, tokens, []TokenType{
		TokenIdent, TokenPlusPlus, TokenIdent, TokenEOF,
	})
}

// containsStr is a simple helper for substring checks.
func containsStr(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- for..in, in, raise, use keyword tests ---

func TestTokenize_ForInKeywords(t *testing.T) {
	tokens, errs := Tokenize("for x in items")
	assertNoErrors(t, errs)
	assertTokenTypes(t, tokens, []TokenType{TokenFor, TokenIdent, TokenIn, TokenIdent, TokenEOF})
}

func TestTokenize_RaiseKeyword(t *testing.T) {
	tokens, errs := Tokenize(`raise "error"`)
	assertNoErrors(t, errs)
	assertTokenTypes(t, tokens, []TokenType{TokenRaise, TokenString, TokenEOF})
}

func TestTokenize_UseKeyword(t *testing.T) {
	tokens, errs := Tokenize("use maintenance")
	assertNoErrors(t, errs)
	assertTokenTypes(t, tokens, []TokenType{TokenUse, TokenIdent, TokenEOF})
}
