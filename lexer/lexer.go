package lexer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xraph/dtl/ast"
)

// TokenType identifies a lexer token.
type TokenType int

const (
	// Special
	TokenEOF TokenType = iota
	TokenNewline
	TokenIndent
	TokenDedent

	// Literals
	TokenInt    // 42
	TokenFloat  // 3.14
	TokenString // "hello"
	TokenIdent  // variable_name
	TokenTrue   // true
	TokenFalse  // false
	TokenNull   // null

	// String interpolation
	TokenStringStart // opening " before first interpolation
	TokenStringPart  // string fragment between interpolations
	TokenStringEnd   // closing " after last interpolation
	TokenInterpStart // { inside string
	TokenInterpEnd   // } inside string

	// Keywords
	TokenFn
	TokenLet
	TokenReturn
	TokenIf
	TokenThen
	TokenElse
	TokenMatch
	TokenWhen
	TokenTry
	TokenCatch
	TokenAnd
	TokenOr
	TokenNot
	TokenQuery
	TokenFor
	TokenIn
	TokenRaise
	TokenUse
	TokenRecord
	TokenTypeKw // 'type' keyword for named type definitions

	// Operators
	TokenPlus       // +
	TokenMinus      // -
	TokenStar       // *
	TokenSlash      // /
	TokenPercent    // %
	TokenEq         // ==
	TokenNe         // !=
	TokenGt         // >
	TokenLt         // <
	TokenGe         // >=
	TokenLe         // <=
	TokenAmpAmp     // &&
	TokenPipePipe   // ||
	TokenBang       // !
	TokenPipe       // |
	TokenArrow      // =>
	TokenThinArrow  // ->
	TokenDotDot     // ..
	TokenQQ         // ??
	TokenQDot       // ?.
	TokenPlusPlus   // ++
	TokenDot        // .
	TokenColon      // :
	TokenColonColon // ::
	TokenQuestion   // ?

	// Delimiters
	TokenLParen   // (
	TokenRParen   // )
	TokenLBracket // [
	TokenRBracket // ]
	TokenLBrace   // {
	TokenRBrace   // }
	TokenComma    // ,
	TokenEllipsis // ...
	TokenAssign   // =

	// Sentinel for token type count
	tokenTypeCount
)

var tokenNames = [tokenTypeCount]string{
	TokenEOF:     "EOF",
	TokenNewline: "NEWLINE",
	TokenIndent:  "INDENT",
	TokenDedent:  "DEDENT",

	TokenInt:    "INT",
	TokenFloat:  "FLOAT",
	TokenString: "STRING",
	TokenIdent:  "IDENT",
	TokenTrue:   "true",
	TokenFalse:  "false",
	TokenNull:   "null",

	TokenStringStart: "STRING_START",
	TokenStringPart:  "STRING_PART",
	TokenStringEnd:   "STRING_END",
	TokenInterpStart: "INTERP_START",
	TokenInterpEnd:   "INTERP_END",

	TokenFn:     "fn",
	TokenLet:    "let",
	TokenReturn: "return",
	TokenIf:     "if",
	TokenThen:   "then",
	TokenElse:   "else",
	TokenMatch:  "match",
	TokenWhen:   "when",
	TokenTry:    "try",
	TokenCatch:  "catch",
	TokenAnd:    "and",
	TokenOr:     "or",
	TokenNot:    "not",
	TokenQuery:  "query",
	TokenFor:    "for",
	TokenIn:     "in",
	TokenRaise:  "raise",
	TokenUse:    "use",
	TokenRecord: "record",
	TokenTypeKw: "type",

	TokenPlus:       "+",
	TokenMinus:      "-",
	TokenStar:       "*",
	TokenSlash:      "/",
	TokenPercent:    "%",
	TokenEq:         "==",
	TokenNe:         "!=",
	TokenGt:         ">",
	TokenLt:         "<",
	TokenGe:         ">=",
	TokenLe:         "<=",
	TokenAmpAmp:     "&&",
	TokenPipePipe:   "||",
	TokenBang:       "!",
	TokenPipe:       "|",
	TokenArrow:      "=>",
	TokenThinArrow:  "->",
	TokenDotDot:     "..",
	TokenQQ:         "??",
	TokenQDot:       "?.",
	TokenPlusPlus:   "++",
	TokenDot:        ".",
	TokenColon:      ":",
	TokenColonColon: "::",
	TokenQuestion:   "?",

	TokenLParen:   "(",
	TokenRParen:   ")",
	TokenLBracket: "[",
	TokenRBracket: "]",
	TokenLBrace:   "{",
	TokenRBrace:   "}",
	TokenComma:    ",",
	TokenEllipsis: "...",
	TokenAssign:   "=",
}

func (t TokenType) String() string {
	if int(t) < len(tokenNames) {
		return tokenNames[t]
	}
	return fmt.Sprintf("Token(%d)", t)
}

// Token is a single lexical token with its source position and value.
type Token struct {
	Type TokenType
	Pos  ast.Position
	Val  string // raw text of the token
}

func (t Token) String() string {
	if t.Val != "" {
		return fmt.Sprintf("%s(%q)", t.Type, t.Val)
	}
	return t.Type.String()
}

// keywords maps reserved words to their token type.
var keywords = map[string]TokenType{
	"fn":     TokenFn,
	"let":    TokenLet,
	"return": TokenReturn,
	"if":     TokenIf,
	"then":   TokenThen,
	"else":   TokenElse,
	"match":  TokenMatch,
	"when":   TokenWhen,
	"try":    TokenTry,
	"catch":  TokenCatch,
	"and":    TokenAnd,
	"or":     TokenOr,
	"not":    TokenNot,
	"true":   TokenTrue,
	"false":  TokenFalse,
	"null":   TokenNull,
	"query":  TokenQuery,
	"for":    TokenFor,
	"in":     TokenIn,
	"raise":  TokenRaise,
	"use":    TokenUse,
	"record": TokenRecord,
	"type":   TokenTypeKw,
}

// LexError records a lexer error with source position.
type LexError struct {
	Pos     ast.Position
	Message string
}

func (e *LexError) Error() string {
	return fmt.Sprintf("line %d, col %d: %s", e.Pos.Line, e.Pos.Column, e.Message)
}

// Lexer tokenizes DTL source code.
type Lexer struct {
	src    []byte
	pos    int // current byte offset
	line   int
	col    int
	tokens []Token
	errors []LexError

	// Indentation tracking
	indentStack []int // stack of indentation levels
	atLineStart bool  // true when we're at the beginning of a line
	braceDepth  int   // track { } for string interpolation
}

// Tokenize converts DTL source into a token slice.
func Tokenize(src string) ([]Token, []LexError) {
	l := &Lexer{
		src:         []byte(src),
		line:        1,
		col:         1,
		indentStack: []int{0},
		atLineStart: true,
	}
	l.run()
	return l.tokens, l.errors
}

func (l *Lexer) run() {
	for !l.atEnd() {
		l.scanToken()
	}
	// Emit remaining DEDENTs
	for len(l.indentStack) > 1 {
		l.emit(TokenDedent, "")
		l.indentStack = l.indentStack[:len(l.indentStack)-1]
	}
	l.emit(TokenEOF, "")
}

func (l *Lexer) scanToken() {
	// Handle beginning-of-line indentation
	if l.atLineStart {
		l.handleIndentation()
		l.atLineStart = false
		if l.atEnd() {
			return
		}
	}

	ch := l.peek()

	// Skip spaces and tabs (not newlines — those are significant)
	if ch == ' ' || ch == '\t' {
		l.advance()
		return
	}

	// Newline
	if ch == '\n' {
		l.emit(TokenNewline, "\n")
		l.advance()
		l.line++
		l.col = 1
		l.atLineStart = true
		return
	}

	// Carriage return (handle \r\n)
	if ch == '\r' {
		l.advance()
		if !l.atEnd() && l.peek() == '\n' {
			l.advance()
		}
		l.emit(TokenNewline, "\n")
		l.line++
		l.col = 1
		l.atLineStart = true
		return
	}

	// Line comment: --
	if ch == '-' && l.peekAt(1) == '-' {
		l.skipLineComment()
		return
	}

	// Block comment: {- ... -}
	if ch == '{' && l.peekAt(1) == '-' {
		l.skipBlockComment()
		return
	}

	// String literal
	if ch == '"' {
		l.scanString()
		return
	}

	// Number
	if ch >= '0' && ch <= '9' {
		l.scanNumber()
		return
	}

	// Identifier or keyword
	if isIdentStart(ch) {
		l.scanIdent()
		return
	}

	// Operators and delimiters
	l.scanOperator()
}

func (l *Lexer) handleIndentation() {
	indent := 0
loop:
	for !l.atEnd() {
		ch := l.peek()
		switch ch {
		case ' ':
			indent++
			l.advance()
		case '\t':
			indent += 4 // treat tab as 4 spaces
			l.advance()
		default:
			break loop
		}
	}

	// Skip blank lines and comment-only lines
	if l.atEnd() || l.peek() == '\n' || l.peek() == '\r' {
		return
	}
	if l.peek() == '-' && l.peekAt(1) == '-' {
		return
	}

	current := l.indentStack[len(l.indentStack)-1]
	if indent > current {
		l.indentStack = append(l.indentStack, indent)
		l.emit(TokenIndent, "")
	} else if indent < current {
		for len(l.indentStack) > 1 && l.indentStack[len(l.indentStack)-1] > indent {
			l.indentStack = l.indentStack[:len(l.indentStack)-1]
			l.emit(TokenDedent, "")
		}
		if l.indentStack[len(l.indentStack)-1] != indent {
			l.addError("inconsistent indentation")
		}
	}
}

func (l *Lexer) scanString() {
	l.advance() // consume opening "
	startPos := l.currentPos()

	var buf strings.Builder
	hasInterp := false

	for !l.atEnd() && l.peek() != '"' {
		ch := l.peek()
		if ch == '\n' {
			l.addError("unterminated string literal")
			return
		}
		// Check for interpolation: {expr}
		if ch == '{' {
			hasInterp = true
			if buf.Len() > 0 || !hasInterp {
				// Emit string part before interpolation
				if len(l.tokens) == 0 || (l.lastTokenType() != TokenStringStart && l.lastTokenType() != TokenInterpEnd) {
					l.emitAt(TokenStringStart, buf.String(), startPos)
				} else {
					l.emitAt(TokenStringPart, buf.String(), startPos)
				}
				buf.Reset()
			} else {
				l.emitAt(TokenStringStart, buf.String(), startPos)
				buf.Reset()
			}
			l.advance() // consume {
			l.emit(TokenInterpStart, "{")
			l.braceDepth++
			l.scanInterpolation()
			startPos = l.currentPos()
			continue
		}

		// Escape sequences
		if ch == '\\' {
			l.advance()
			if l.atEnd() {
				l.addError("unterminated escape sequence")
				return
			}
			esc := l.peek()
			switch esc {
			case 'n':
				buf.WriteByte('\n')
			case 't':
				buf.WriteByte('\t')
			case 'r':
				buf.WriteByte('\r')
			case '\\':
				buf.WriteByte('\\')
			case '"':
				buf.WriteByte('"')
			case '{':
				buf.WriteByte('{')
			default:
				buf.WriteByte('\\')
				buf.WriteByte(esc)
			}
			l.advance()
			continue
		}

		buf.WriteByte(ch)
		l.advance()
	}

	if l.atEnd() {
		l.addError("unterminated string literal")
		return
	}
	l.advance() // consume closing "

	if hasInterp {
		l.emitAt(TokenStringEnd, buf.String(), startPos)
	} else {
		l.emitAt(TokenString, buf.String(), startPos)
	}
}

func (l *Lexer) scanInterpolation() {
	depth := 1
	for !l.atEnd() && depth > 0 {
		ch := l.peek()
		if ch == '}' {
			depth--
			if depth == 0 {
				l.emit(TokenInterpEnd, "}")
				l.advance()
				l.braceDepth--
				return
			}
		}
		if ch == '{' {
			depth++
		}
		l.scanToken()
	}
	if depth > 0 {
		l.addError("unterminated string interpolation")
	}
}

func (l *Lexer) scanNumber() {
	start := l.pos
	startPos := l.currentPos()
	isFloat := false

	for !l.atEnd() && l.peek() >= '0' && l.peek() <= '9' {
		l.advance()
	}
	// Check for decimal point (but not .. which is range operator)
	if !l.atEnd() && l.peek() == '.' && l.peekAt(1) != '.' {
		next := l.peekAt(1)
		if next >= '0' && next <= '9' {
			isFloat = true
			l.advance() // consume .
			for !l.atEnd() && l.peek() >= '0' && l.peek() <= '9' {
				l.advance()
			}
		}
	}

	val := string(l.src[start:l.pos])
	if isFloat {
		l.emitAt(TokenFloat, val, startPos)
	} else {
		l.emitAt(TokenInt, val, startPos)
	}
}

func (l *Lexer) scanIdent() {
	start := l.pos
	startPos := l.currentPos()

	for !l.atEnd() && isIdentPart(l.peek()) {
		l.advance()
	}

	val := string(l.src[start:l.pos])
	if tt, ok := keywords[val]; ok {
		l.emitAt(tt, val, startPos)
	} else {
		l.emitAt(TokenIdent, val, startPos)
	}
}

func (l *Lexer) scanOperator() {
	pos := l.currentPos()
	ch := l.peek()
	next := l.peekAt(1)

	switch ch {
	case '+':
		if next == '+' {
			l.advance()
			l.advance()
			l.emitAt(TokenPlusPlus, "++", pos)
		} else {
			l.advance()
			l.emitAt(TokenPlus, "+", pos)
		}
	case '-':
		if next == '>' {
			l.advance()
			l.advance()
			l.emitAt(TokenThinArrow, "->", pos)
		} else {
			l.advance()
			l.emitAt(TokenMinus, "-", pos)
		}
	case '*':
		l.advance()
		l.emitAt(TokenStar, "*", pos)
	case '/':
		l.advance()
		l.emitAt(TokenSlash, "/", pos)
	case '%':
		l.advance()
		l.emitAt(TokenPercent, "%", pos)
	case '=':
		switch next {
		case '=':
			l.advance()
			l.advance()
			l.emitAt(TokenEq, "==", pos)
		case '>':
			l.advance()
			l.advance()
			l.emitAt(TokenArrow, "=>", pos)
		default:
			l.advance()
			l.emitAt(TokenAssign, "=", pos)
		}
	case '!':
		if next == '=' {
			l.advance()
			l.advance()
			l.emitAt(TokenNe, "!=", pos)
		} else {
			l.advance()
			l.emitAt(TokenBang, "!", pos)
		}
	case '>':
		if next == '=' {
			l.advance()
			l.advance()
			l.emitAt(TokenGe, ">=", pos)
		} else {
			l.advance()
			l.emitAt(TokenGt, ">", pos)
		}
	case '<':
		if next == '=' {
			l.advance()
			l.advance()
			l.emitAt(TokenLe, "<=", pos)
		} else {
			l.advance()
			l.emitAt(TokenLt, "<", pos)
		}
	case '&':
		if next == '&' {
			l.advance()
			l.advance()
			l.emitAt(TokenAmpAmp, "&&", pos)
		} else {
			l.advance()
			l.addError(fmt.Sprintf("unexpected character: %c", ch))
		}
	case '|':
		if next == '|' {
			l.advance()
			l.advance()
			l.emitAt(TokenPipePipe, "||", pos)
		} else {
			l.advance()
			l.emitAt(TokenPipe, "|", pos)
		}
	case '?':
		switch next {
		case '?':
			l.advance()
			l.advance()
			l.emitAt(TokenQQ, "??", pos)
		case '.':
			l.advance()
			l.advance()
			l.emitAt(TokenQDot, "?.", pos)
		default:
			l.advance()
			l.emitAt(TokenQuestion, "?", pos)
		}
	case '.':
		if next == '.' {
			third := l.peekAt(2)
			if third == '.' {
				l.advance()
				l.advance()
				l.advance()
				l.emitAt(TokenEllipsis, "...", pos)
			} else {
				l.advance()
				l.advance()
				l.emitAt(TokenDotDot, "..", pos)
			}
		} else {
			l.advance()
			l.emitAt(TokenDot, ".", pos)
		}
	case ':':
		if next == ':' {
			l.advance()
			l.advance()
			l.emitAt(TokenColonColon, "::", pos)
		} else {
			l.advance()
			l.emitAt(TokenColon, ":", pos)
		}
	case '(':
		l.advance()
		l.emitAt(TokenLParen, "(", pos)
	case ')':
		l.advance()
		l.emitAt(TokenRParen, ")", pos)
	case '[':
		l.advance()
		l.emitAt(TokenLBracket, "[", pos)
	case ']':
		l.advance()
		l.emitAt(TokenRBracket, "]", pos)
	case '{':
		l.advance()
		l.emitAt(TokenLBrace, "{", pos)
	case '}':
		l.advance()
		l.emitAt(TokenRBrace, "}", pos)
	case ',':
		l.advance()
		l.emitAt(TokenComma, ",", pos)
	case '$':
		// $ prefix for parameter references in query context — treat as ident
		l.advance()
		if !l.atEnd() && isIdentStart(l.peek()) {
			start := l.pos
			for !l.atEnd() && isIdentPart(l.peek()) {
				l.advance()
			}
			val := "$" + string(l.src[start:l.pos])
			l.emitAt(TokenIdent, val, pos)
		} else {
			l.addError("expected identifier after $")
		}
	default:
		l.advance()
		l.addError(fmt.Sprintf("unexpected character: %c", ch))
	}
}

func (l *Lexer) skipLineComment() {
	l.advance() // first -
	l.advance() // second -
	for !l.atEnd() && l.peek() != '\n' {
		l.advance()
	}
}

func (l *Lexer) skipBlockComment() {
	l.advance() // {
	l.advance() // -
	depth := 1
	for !l.atEnd() && depth > 0 {
		if l.peek() == '{' && l.peekAt(1) == '-' {
			depth++
			l.advance()
			l.advance()
		} else if l.peek() == '-' && l.peekAt(1) == '}' {
			depth--
			l.advance()
			l.advance()
		} else {
			if l.peek() == '\n' {
				l.line++
				l.col = 0 // will be 1 after advance
			}
			l.advance()
		}
	}
	if depth > 0 {
		l.addError("unterminated block comment")
	}
}

// --- Low-level helpers ---

func (l *Lexer) atEnd() bool {
	return l.pos >= len(l.src)
}

func (l *Lexer) peek() byte {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *Lexer) peekAt(offset int) byte {
	idx := l.pos + offset
	if idx >= len(l.src) || idx < 0 {
		return 0
	}
	return l.src[idx]
}

func (l *Lexer) advance() {
	if l.pos < len(l.src) {
		l.pos++
		l.col++
	}
}

func (l *Lexer) currentPos() ast.Position {
	return ast.Position{Line: l.line, Column: l.col}
}

func (l *Lexer) emit(tt TokenType, val string) {
	l.tokens = append(l.tokens, Token{Type: tt, Pos: l.currentPos(), Val: val})
}

func (l *Lexer) emitAt(tt TokenType, val string, pos ast.Position) {
	l.tokens = append(l.tokens, Token{Type: tt, Pos: pos, Val: val})
}

func (l *Lexer) addError(msg string) {
	l.errors = append(l.errors, LexError{Pos: l.currentPos(), Message: msg})
}

func (l *Lexer) lastTokenType() TokenType {
	if len(l.tokens) == 0 {
		return TokenEOF
	}
	return l.tokens[len(l.tokens)-1].Type
}

func isIdentStart(ch byte) bool {
	if ch >= utf8.RuneSelf {
		r, _ := utf8.DecodeRune([]byte{ch})
		return unicode.IsLetter(r) || r == '_'
	}
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentPart(ch byte) bool {
	if ch >= utf8.RuneSelf {
		r, _ := utf8.DecodeRune([]byte{ch})
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
	}
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}
