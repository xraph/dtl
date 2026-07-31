package parser

import (
	"fmt"
	"strconv"

	"github.com/xraph/dtl/ast"
	"github.com/xraph/dtl/lexer"
)

// ParseError records a parser error with source position.
type ParseError struct {
	Pos     ast.Position `json:"pos"`
	Message string       `json:"message"`
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d, col %d: %s", e.Pos.Line, e.Pos.Column, e.Message)
}

// Parser converts a token stream into an AST.
type Parser struct {
	tokens  []lexer.Token
	pos     int
	errors  []ParseError
	current lexer.Token
}

// Parse tokenizes source and parses a function definition.
func Parse(source string) (*ast.FnAST, []ParseError) {
	tokens, lexErrs := lexer.Tokenize(source)

	p := &Parser{tokens: tokens}
	// Convert lex errors to parse errors
	for _, le := range lexErrs {
		p.errors = append(p.errors, ParseError{Pos: le.Pos, Message: le.Message})
	}

	if len(tokens) == 0 {
		p.errors = append(p.errors, ParseError{
			Pos:     ast.Position{Line: 1, Column: 1},
			Message: "empty source",
		})
		return nil, p.errors
	}
	p.current = tokens[0]

	fn := p.parseFunction()
	if len(p.errors) > 0 {
		return fn, p.errors
	}
	return fn, nil
}

// ParseExpression parses a standalone expression (for inline execute).
func ParseExpression(source string) (ast.ExprNode, []ParseError) {
	tokens, lexErrs := lexer.Tokenize(source)
	p := &Parser{tokens: tokens}
	for _, le := range lexErrs {
		p.errors = append(p.errors, ParseError{Pos: le.Pos, Message: le.Message})
	}
	if len(tokens) == 0 {
		p.errors = append(p.errors, ParseError{
			Pos:     ast.Position{Line: 1, Column: 1},
			Message: "empty expression",
		})
		return nil, p.errors
	}
	p.current = tokens[0]
	p.skipNewlines()
	expr := p.parseExpr()
	if len(p.errors) > 0 {
		return expr, p.errors
	}
	return expr, nil
}

// --- Token Navigation ---

func (p *Parser) peek() lexer.Token {
	return p.current
}

func (p *Parser) advance() lexer.Token {
	tok := p.current
	p.pos++
	if p.pos < len(p.tokens) {
		p.current = p.tokens[p.pos]
	} else {
		p.current = lexer.Token{Type: lexer.TokenEOF}
	}
	return tok
}

func (p *Parser) expect(tt lexer.TokenType) lexer.Token {
	if p.current.Type != tt {
		p.addError(fmt.Sprintf("expected %s, got %s", friendlyTokenName(tt), friendlyTokenName(p.current.Type)))
		return p.current
	}
	return p.advance()
}

// expectName consumes a name token where the grammar expects an identifier.
// `type` is a contextual keyword — it introduces a type definition only at
// the head of a declaration, so in name positions (parameter names, record
// field names) it is just an ordinary identifier. Treating it as such keeps
// pack-shipped DTL like `fn f(type: string)` valid.
func (p *Parser) expectName() lexer.Token {
	if p.current.Type == lexer.TokenTypeKw {
		return p.advance()
	}
	return p.expect(lexer.TokenIdent)
}

// friendlyTokenName converts technical token names to human-readable descriptions.
func friendlyTokenName(tt lexer.TokenType) string {
	switch tt {
	case lexer.TokenIdent:
		return "a name"
	case lexer.TokenInt:
		return "a number"
	case lexer.TokenFloat:
		return "a decimal number"
	case lexer.TokenString:
		return "a text string"
	case lexer.TokenLParen:
		return "'('"
	case lexer.TokenRParen:
		return "')'"
	case lexer.TokenLBracket:
		return "'['"
	case lexer.TokenRBracket:
		return "']'"
	case lexer.TokenLBrace:
		return "'{'"
	case lexer.TokenRBrace:
		return "'}'"
	case lexer.TokenColon:
		return "':'"
	case lexer.TokenComma:
		return "','"
	case lexer.TokenAssign:
		return "'='"
	case lexer.TokenArrow:
		return "'=>'"
	case lexer.TokenThinArrow:
		return "'->'"
	case lexer.TokenEOF:
		return "end of input"
	case lexer.TokenNewline:
		return "end of line"
	default:
		return tt.String()
	}
}

func (p *Parser) match(types ...lexer.TokenType) bool {
	for _, tt := range types {
		if p.current.Type == tt {
			return true
		}
	}
	return false
}

func (p *Parser) skipNewlines() {
	for p.current.Type == lexer.TokenNewline {
		p.advance()
	}
}

// skipLayout consumes newlines and indentation tokens. Inside brace/bracket
// delimited literals indentation is insignificant, but the lexer still emits
// INDENT/DEDENT for its block tracking — multi-line object and array literals
// must ignore them.
func (p *Parser) skipLayout() {
	for p.current.Type == lexer.TokenNewline ||
		p.current.Type == lexer.TokenIndent ||
		p.current.Type == lexer.TokenDedent {
		p.advance()
	}
}

func (p *Parser) addError(msg string) {
	p.errors = append(p.errors, ParseError{Pos: p.current.Pos, Message: msg})
}

// --- Function Declaration ---

func (p *Parser) parseFunction() *ast.FnAST {
	p.skipNewlines()

	// Parse local type definitions before the fn keyword.
	var typeDefs []ast.TypeDefStmt
	for p.current.Type == lexer.TokenTypeKw {
		td := p.parseTypeDef()
		typeDefs = append(typeDefs, td)
		p.skipNewlines()
	}

	fnTok := p.expect(lexer.TokenFn)
	fn := &ast.FnAST{Pos: fnTok.Pos, TypeDefs: typeDefs}

	// Function name (may include :: namespacing, but that's encoded in the store fullName)
	nameTok := p.expect(lexer.TokenIdent)
	fn.Name = nameTok.Val

	// Parameters
	p.expect(lexer.TokenLParen)
	fn.Params = p.parseParams()
	p.expect(lexer.TokenRParen)

	// Return type
	p.expect(lexer.TokenThinArrow)
	fn.ReturnType = p.parseType()

	p.skipNewlines()

	// Body: one-liner (=>) or block (:)
	switch p.current.Type {
	case lexer.TokenArrow:
		p.advance()
		p.skipNewlines()
		// If the next token is INDENT, treat as block body (=> can be used interchangeably with :)
		if p.current.Type == lexer.TokenIndent {
			fn.Body = p.parseBlock()
		} else {
			fn.IsOneLiner = true
			expr := p.parseExpr()
			fn.Body = []ast.StmtNode{&ast.ExprStmt{Pos: expr.ExprPos(), Expr: expr}}
		}
	case lexer.TokenColon:
		p.advance()
		fn.Body = p.parseBlock()
	default:
		p.addError("expected => or : after return type")
	}

	return fn
}

func (p *Parser) parseParams() []ast.ParamNode {
	params := make([]ast.ParamNode, 0, 4)
	for {
		// Parameter lists may span lines (the `(` … `)` is brace-delimited, so
		// the newlines/indentation the lexer emits inside it are insignificant).
		p.skipLayout()
		if p.current.Type == lexer.TokenRParen {
			break
		}
		param := p.parseParam()
		params = append(params, param)
		p.skipLayout()
		if p.current.Type != lexer.TokenComma {
			break
		}
		p.advance() // consume comma
	}
	return params
}

func (p *Parser) parseParam() ast.ParamNode {
	param := ast.ParamNode{Pos: p.current.Pos}

	// Variadic: ...name
	if p.current.Type == lexer.TokenEllipsis {
		p.advance()
		param.Variadic = true
	}

	nameTok := p.expectName()
	param.Name = nameTok.Val

	p.expect(lexer.TokenColon)
	param.Type = p.parseType()

	// Optional default value
	if p.current.Type == lexer.TokenAssign {
		p.advance()
		param.Default = p.parseExpr()
	}

	return param
}

func (p *Parser) parseType() ast.TypeNode {
	// Record type: record { name: string, age: int }
	if p.current.Type == lexer.TokenRecord {
		return p.parseRecordType()
	}

	tn := ast.TypeNode{Pos: p.current.Pos}
	nameTok := p.expect(lexer.TokenIdent)
	tn.Name = nameTok.Val

	// Check for array suffix []
	if p.current.Type == lexer.TokenLBracket {
		p.advance()
		p.expect(lexer.TokenRBracket)
		tn.IsArray = true
	}
	return tn
}

// parseRecordType parses: record { name: type, name?: type, ... }
func (p *Parser) parseRecordType() ast.TypeNode {
	tn := ast.TypeNode{Pos: p.current.Pos, Name: "record"}
	p.advance() // consume 'record'

	p.expect(lexer.TokenLBrace)
	p.skipNewlines()

	fields := make([]ast.RecordField, 0, 4)
	for p.current.Type != lexer.TokenRBrace && p.current.Type != lexer.TokenEOF {
		field := ast.RecordField{Pos: p.current.Pos}
		nameTok := p.expectName()
		field.Name = nameTok.Val

		// Optional marker: name?
		if p.current.Type == lexer.TokenQuestion {
			field.Optional = true
			p.advance() // consume ?
		}

		p.expect(lexer.TokenColon)
		p.skipNewlines()
		field.Type = p.parseType()
		fields = append(fields, field)

		p.skipNewlines()
		if p.current.Type == lexer.TokenComma {
			p.advance()
			p.skipNewlines()
		} else {
			break
		}
	}

	p.expect(lexer.TokenRBrace)
	tn.Fields = fields

	// Check for array suffix []
	if p.current.Type == lexer.TokenLBracket {
		p.advance()
		p.expect(lexer.TokenRBracket)
		tn.IsArray = true
	}
	return tn
}

// parseTypeDef parses: type Name = record { ... }
func (p *Parser) parseTypeDef() ast.TypeDefStmt {
	pos := p.current.Pos
	p.advance() // consume 'type'

	nameTok := p.expect(lexer.TokenIdent)
	p.expect(lexer.TokenAssign)
	p.skipNewlines()

	typNode := p.parseType()
	return ast.TypeDefStmt{Pos: pos, Name: nameTok.Val, Type: typNode}
}

// --- Block Parsing ---

func (p *Parser) parseBlock() []ast.StmtNode {
	stmts := make([]ast.StmtNode, 0, 4)

	p.skipNewlines()

	// Expect INDENT
	if p.current.Type == lexer.TokenIndent {
		p.advance()
	}

	for p.current.Type != lexer.TokenDedent && p.current.Type != lexer.TokenEOF {
		p.skipNewlines()
		if p.current.Type == lexer.TokenDedent || p.current.Type == lexer.TokenEOF {
			break
		}
		stmt := p.parseStmt()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
		p.skipNewlines()
	}

	if p.current.Type == lexer.TokenDedent {
		p.advance()
	}

	return stmts
}

func (p *Parser) parseStmt() ast.StmtNode {
	switch p.current.Type {
	case lexer.TokenLet:
		return p.parseLetStmt()
	case lexer.TokenReturn:
		return p.parseReturnStmt()
	case lexer.TokenUse:
		return p.parseUseStmt()
	default:
		expr := p.parseExpr()
		if expr == nil {
			return nil
		}
		return &ast.ExprStmt{Pos: expr.ExprPos(), Expr: expr}
	}
}

func (p *Parser) parseUseStmt() *ast.UseStmt {
	useTok := p.advance() // consume 'use'
	nameTok := p.expect(lexer.TokenIdent)
	namespace := nameTok.Val

	// Pack ids are dotted — com.example.core.shifts — and '.' is not an
	// identifier character, so the lexer hands us IDENT (DOT IDENT)*. Joining
	// them here is what lets the executor build the app:<packID>::<name> key
	// the function registry actually uses.
	for p.current.Type == lexer.TokenDot {
		p.advance() // consume '.'
		part := p.expect(lexer.TokenIdent)
		namespace = namespace + "." + part.Val
	}

	return &ast.UseStmt{Pos: useTok.Pos, Namespace: namespace}
}

func (p *Parser) parseLetStmt() *ast.LetStmt {
	letTok := p.advance() // consume 'let'
	nameTok := p.expect(lexer.TokenIdent)
	p.expect(lexer.TokenAssign)
	p.skipNewlines()
	value := p.parseExpr()
	return &ast.LetStmt{Pos: letTok.Pos, Name: nameTok.Val, Value: value}
}

func (p *Parser) parseReturnStmt() *ast.ReturnStmt {
	retTok := p.advance() // consume 'return'
	value := p.parseExpr()
	return &ast.ReturnStmt{Pos: retTok.Pos, Value: value}
}

// --- Expression Parsing (Pratt-style precedence climbing) ---

func (p *Parser) parseExpr() ast.ExprNode {
	return p.parsePipe()
}

// Pipe has the lowest precedence: expr | func(args)
func (p *Parser) parsePipe() ast.ExprNode {
	left := p.parseCoalesce()

	for p.current.Type == lexer.TokenPipe {
		p.advance()
		p.skipNewlines()
		pos := p.current.Pos

		// The right side of a pipe is a function call
		// Could be: ident(args) or just ident()
		if p.current.Type == lexer.TokenIdent {
			name := p.parseFunctionName()

			var args []ast.ExprNode
			if p.current.Type == lexer.TokenLParen {
				p.advance()
				args = p.parseArgList()
				p.expect(lexer.TokenRParen)
			}
			left = &ast.PipeExpr{
				Pos:      pos,
				Input:    left,
				Function: name,
				Args:     args,
			}
		} else {
			// Pipe into lambda shorthand: values | > 0 means values | filter(x => x > 0)
			// For now, parse any expression as a lambda body
			expr := p.parseCoalesce()
			left = &ast.PipeExpr{
				Pos:      pos,
				Input:    left,
				Function: "",
				Args:     []ast.ExprNode{expr},
			}
		}
	}
	return left
}

func (p *Parser) parseFunctionName() string {
	// Handle namespaced names: system::math::clamp, team:analytics::score
	name := p.advance().Val

	// Handle prefix:scope namespaces (team:analytics, user:john, app:myapp).
	// A single colon followed by ident then :: indicates a scoped namespace prefix.
	// We use lookahead to avoid consuming colons meant for type annotations.
	if p.current.Type == lexer.TokenColon {
		savedPos, savedCurrent := p.pos, p.current
		p.advance() // consume :
		if p.current.Type == lexer.TokenIdent {
			scopeTok := p.advance() // consume ident (e.g., "analytics")
			if p.current.Type == lexer.TokenColonColon {
				// Confirmed: team:analytics:: pattern
				name += ":" + scopeTok.Val
			} else {
				// Not a namespace pattern — restore parser state
				p.pos, p.current = savedPos, savedCurrent
			}
		} else {
			// Colon not followed by ident — restore
			p.pos, p.current = savedPos, savedCurrent
		}
	}

	// Handle :: namespace separators
	for p.current.Type == lexer.TokenColonColon {
		p.advance()
		// A namespaced segment is normally an identifier, but a few reserved
		// keywords (notably `query`) are also valid as the trailing name —
		// e.g. `dataset::query` is grammar-level sugar for the keyword form.
		if p.current.Type == lexer.TokenIdent || p.current.Type == lexer.TokenQuery {
			next := p.advance()
			name += "::" + next.Val
		} else {
			p.expect(lexer.TokenIdent) // emits "expected a name" error
			break
		}
	}
	return name
}

// ?? (null coalescing)
func (p *Parser) parseCoalesce() ast.ExprNode {
	left := p.parseOr()
	for p.current.Type == lexer.TokenQQ {
		p.advance()
		p.skipNewlines()
		right := p.parseOr()
		left = &ast.CoalesceExpr{Pos: left.ExprPos(), Left: left, Right: right}
	}
	return left
}

// || / or
func (p *Parser) parseOr() ast.ExprNode {
	left := p.parseAnd()
	for p.current.Type == lexer.TokenPipePipe || p.current.Type == lexer.TokenOr {
		op := p.advance()
		p.skipNewlines()
		right := p.parseAnd()
		left = &ast.BinaryExpr{Pos: op.Pos, Op: "||", Left: left, Right: right}
	}
	return left
}

// && / and
func (p *Parser) parseAnd() ast.ExprNode {
	left := p.parseNot()
	for p.current.Type == lexer.TokenAmpAmp || p.current.Type == lexer.TokenAnd {
		op := p.advance()
		p.skipNewlines()
		right := p.parseNot()
		left = &ast.BinaryExpr{Pos: op.Pos, Op: "&&", Left: left, Right: right}
	}
	return left
}

// ! / not (unary, right-associative)
func (p *Parser) parseNot() ast.ExprNode {
	if p.current.Type == lexer.TokenBang || p.current.Type == lexer.TokenNot {
		op := p.advance()
		operand := p.parseNot()
		return &ast.UnaryExpr{Pos: op.Pos, Op: "!", Operand: operand}
	}
	return p.parseComparison()
}

// ==, !=, <, >, <=, >=, in
func (p *Parser) parseComparison() ast.ExprNode {
	left := p.parseAddition()
	if p.match(lexer.TokenEq, lexer.TokenNe, lexer.TokenGt, lexer.TokenLt, lexer.TokenGe, lexer.TokenLe) {
		op := p.advance()
		p.skipNewlines()
		right := p.parseAddition()
		left = &ast.BinaryExpr{Pos: op.Pos, Op: op.Val, Left: left, Right: right}
	} else if p.current.Type == lexer.TokenIn {
		inTok := p.advance()
		p.skipNewlines()
		collection := p.parseAddition()
		left = &ast.InExpr{Pos: inTok.Pos, Value: left, Collection: collection}
	}
	return left
}

// +, -, ++ (string concat)
func (p *Parser) parseAddition() ast.ExprNode {
	left := p.parseMultiply()
	for p.match(lexer.TokenPlus, lexer.TokenMinus, lexer.TokenPlusPlus) {
		op := p.advance()
		p.skipNewlines()
		right := p.parseMultiply()
		left = &ast.BinaryExpr{Pos: op.Pos, Op: op.Val, Left: left, Right: right}
	}
	return left
}

// *, /, %
func (p *Parser) parseMultiply() ast.ExprNode {
	left := p.parseUnary()
	for p.match(lexer.TokenStar, lexer.TokenSlash, lexer.TokenPercent) {
		op := p.advance()
		p.skipNewlines()
		right := p.parseUnary()
		left = &ast.BinaryExpr{Pos: op.Pos, Op: op.Val, Left: left, Right: right}
	}
	return left
}

// Unary: -x, !x
func (p *Parser) parseUnary() ast.ExprNode {
	if p.current.Type == lexer.TokenMinus {
		op := p.advance()
		operand := p.parseUnary()
		return &ast.UnaryExpr{Pos: op.Pos, Op: "-", Operand: operand}
	}
	if p.current.Type == lexer.TokenBang {
		op := p.advance()
		operand := p.parseUnary()
		return &ast.UnaryExpr{Pos: op.Pos, Op: "!", Operand: operand}
	}
	return p.parsePostfix()
}

// Postfix: field access, optional chaining, indexing, function call
func (p *Parser) parsePostfix() ast.ExprNode {
	left := p.parsePrimary()

	for {
		switch p.current.Type {
		case lexer.TokenDot:
			p.advance()
			fieldTok := p.expect(lexer.TokenIdent)
			left = &ast.FieldAccessExpr{
				Pos: fieldTok.Pos, Object: left, Field: fieldTok.Val, Optional: false,
			}
		case lexer.TokenQDot:
			p.advance()
			fieldTok := p.expect(lexer.TokenIdent)
			left = &ast.FieldAccessExpr{
				Pos: fieldTok.Pos, Object: left, Field: fieldTok.Val, Optional: true,
			}
		case lexer.TokenLBracket:
			pos := p.current.Pos
			p.advance()
			index := p.parseExpr()
			p.expect(lexer.TokenRBracket)
			left = &ast.IndexExpr{Pos: pos, Object: left, Index: index}
		case lexer.TokenLParen:
			// Function call on an expression (e.g., result of field access)
			if ident, ok := left.(*ast.IdentExpr); ok {
				pos := ident.Pos
				p.advance()
				args := p.parseArgList()
				p.expect(lexer.TokenRParen)
				left = &ast.FnCallExpr{Pos: pos, Name: ident.Name, Args: args}
			} else {
				return left
			}
		default:
			return left
		}
	}
}

// --- Primary Expressions ---

func (p *Parser) parsePrimary() ast.ExprNode {
	switch p.current.Type {
	case lexer.TokenInt:
		return p.parseInt()
	case lexer.TokenFloat:
		return p.parseFloat()
	case lexer.TokenString:
		return p.parseString()
	case lexer.TokenStringStart:
		return p.parseInterpolatedString()
	case lexer.TokenTrue:
		tok := p.advance()
		return &ast.LiteralExpr{Pos: tok.Pos, Value: true, Type: "bool"}
	case lexer.TokenFalse:
		tok := p.advance()
		return &ast.LiteralExpr{Pos: tok.Pos, Value: false, Type: "bool"}
	case lexer.TokenNull:
		tok := p.advance()
		return &ast.LiteralExpr{Pos: tok.Pos, Value: nil, Type: "null"}
	case lexer.TokenIdent:
		return p.parseIdentOrCall()
	case lexer.TokenLParen:
		return p.parseGroupOrLambda()
	case lexer.TokenLBracket:
		return p.parseArray()
	case lexer.TokenLBrace:
		return p.parseObject()
	case lexer.TokenIf:
		return p.parseIf()
	case lexer.TokenMatch:
		return p.parseMatch()
	case lexer.TokenTry:
		return p.parseTry()
	case lexer.TokenQuery:
		return p.parseQuery()
	case lexer.TokenFor:
		return p.parseFor()
	case lexer.TokenRaise:
		return p.parseRaise()
	case lexer.TokenFn:
		return p.parseFnLambda()
	default:
		p.addError(fmt.Sprintf("unexpected token: %s", p.current.Type))
		p.advance()
		return &ast.LiteralExpr{Pos: p.current.Pos, Value: nil, Type: "null"}
	}
}

func (p *Parser) parseInt() ast.ExprNode {
	tok := p.advance()
	val, err := strconv.ParseInt(tok.Val, 10, 64)
	if err != nil {
		p.addError(fmt.Sprintf("invalid integer: %s", tok.Val))
		return &ast.LiteralExpr{Pos: tok.Pos, Value: int64(0), Type: "int"}
	}
	return &ast.LiteralExpr{Pos: tok.Pos, Value: val, Type: "int"}
}

func (p *Parser) parseFloat() ast.ExprNode {
	tok := p.advance()
	val, err := strconv.ParseFloat(tok.Val, 64)
	if err != nil {
		p.addError(fmt.Sprintf("invalid float: %s", tok.Val))
		return &ast.LiteralExpr{Pos: tok.Pos, Value: 0.0, Type: "float"}
	}
	return &ast.LiteralExpr{Pos: tok.Pos, Value: val, Type: "float"}
}

func (p *Parser) parseString() ast.ExprNode {
	tok := p.advance()
	return &ast.LiteralExpr{Pos: tok.Pos, Value: tok.Val, Type: "string"}
}

func (p *Parser) parseInterpolatedString() ast.ExprNode {
	startTok := p.advance() // consume STRING_START
	parts := make([]ast.ExprNode, 0, 4)

	// Add the initial string part if non-empty
	if startTok.Val != "" {
		parts = append(parts, &ast.LiteralExpr{Pos: startTok.Pos, Value: startTok.Val, Type: "string"})
	}

	for p.current.Type != lexer.TokenStringEnd && p.current.Type != lexer.TokenEOF {
		handled := true
		switch p.current.Type {
		case lexer.TokenInterpStart:
			p.advance() // consume {
			expr := p.parseExpr()
			parts = append(parts, expr)
			if p.current.Type == lexer.TokenInterpEnd {
				p.advance() // consume }
			}
		case lexer.TokenStringPart:
			tok := p.advance()
			if tok.Val != "" {
				parts = append(parts, &ast.LiteralExpr{Pos: tok.Pos, Value: tok.Val, Type: "string"})
			}
		default:
			handled = false
		}
		if !handled {
			break
		}
	}

	// Final string part
	if p.current.Type == lexer.TokenStringEnd {
		endTok := p.advance()
		if endTok.Val != "" {
			parts = append(parts, &ast.LiteralExpr{Pos: endTok.Pos, Value: endTok.Val, Type: "string"})
		}
	}

	return &ast.InterpolatedStringExpr{Pos: startTok.Pos, Parts: parts}
}

func (p *Parser) parseIdentOrCall() ast.ExprNode {
	tok := p.peek()
	name := p.parseFunctionName()

	// Check if this is a function call
	if p.current.Type == lexer.TokenLParen {
		// `dataset::query(...)` is grammar-level sugar for the `query`
		// keyword when called with a single dataset-name argument — it
		// produces the same QueryExpr AST node so the pipe-chain
		// optimization applies uniformly to both spellings. The 2-arg
		// form `dataset::query(name, dsl)` falls through to the regular
		// FnCallExpr path because QueryExpr doesn't carry a DSL slot.
		p.advance() // consume (
		args := p.parseArgList()
		p.expect(lexer.TokenRParen)
		if name == "dataset::query" && len(args) == 1 {
			return &ast.QueryExpr{Pos: tok.Pos, Dataset: args[0]}
		}
		return &ast.FnCallExpr{Pos: tok.Pos, Name: name, Args: args}
	}

	return &ast.IdentExpr{Pos: tok.Pos, Name: name}
}

func (p *Parser) parseGroupOrLambda() ast.ExprNode {
	pos := p.current.Pos
	p.advance() // consume (

	// Empty parens: might be () => expr (no-arg lambda)
	if p.current.Type == lexer.TokenRParen {
		p.advance()
		if p.current.Type == lexer.TokenArrow {
			p.advance()
			p.skipLayout()
			body := p.parseExpr()
			return &ast.LambdaExpr{Pos: pos, Params: nil, Body: body}
		}
		// Just an empty grouping?? treat as null
		return &ast.LiteralExpr{Pos: pos, Value: nil, Type: "null"}
	}

	// Try to parse as lambda: (x, y) => expr
	// Save position to backtrack if it's just a grouped expression
	savedPos := p.pos
	savedCurrent := p.current
	savedErrors := len(p.errors)

	// Try parsing identifiers separated by commas
	var identNames []string
	isLambda := true
	for {
		if p.current.Type != lexer.TokenIdent {
			isLambda = false
			break
		}
		identNames = append(identNames, p.advance().Val)
		if p.current.Type == lexer.TokenComma {
			p.advance()
			continue
		}
		break
	}

	if isLambda && p.current.Type == lexer.TokenRParen {
		p.advance()
		if p.current.Type == lexer.TokenArrow {
			p.advance()
			p.skipLayout()
			body := p.parseExpr()
			return &ast.LambdaExpr{Pos: pos, Params: identNames, Body: body}
		}
		// Single ident in parens with no arrow — it's a grouped expression
		if len(identNames) == 1 {
			return &ast.IdentExpr{Pos: pos, Name: identNames[0]}
		}
	}

	// Not a lambda — backtrack and parse as grouped expression
	p.pos = savedPos
	p.current = savedCurrent
	p.errors = p.errors[:savedErrors]

	expr := p.parseExpr()
	p.expect(lexer.TokenRParen)
	return expr
}

// parseFnLambda parses an anonymous function expression: fn (a, b) => body.
// Same shape as the paren lambda (x, y) => body, just with the fn keyword —
// how pack DTL writes higher-order args, e.g. list::map(xs, fn (x) => ...).
// The body may sit on an indented next line, so layout tokens are skipped.
func (p *Parser) parseFnLambda() ast.ExprNode {
	fnTok := p.advance() // consume fn
	p.expect(lexer.TokenLParen)
	var params []string
	for p.current.Type == lexer.TokenIdent {
		params = append(params, p.advance().Val)
		if p.current.Type == lexer.TokenComma {
			p.advance()
			continue
		}
		break
	}
	p.expect(lexer.TokenRParen)
	p.expect(lexer.TokenArrow)
	p.skipLayout()
	body := p.parseExpr()
	return &ast.LambdaExpr{Pos: fnTok.Pos, Params: params, Body: body}
}

func (p *Parser) parseArray() ast.ExprNode {
	pos := p.current.Pos
	p.advance() // consume [
	p.skipLayout()

	elements := make([]ast.ExprNode, 0, 8)
	for p.current.Type != lexer.TokenRBracket && p.current.Type != lexer.TokenEOF {
		elem := p.parseExpr()
		elements = append(elements, elem)
		p.skipLayout()
		if p.current.Type == lexer.TokenComma {
			p.advance()
			p.skipLayout()
		} else {
			break
		}
	}
	p.expect(lexer.TokenRBracket)
	return &ast.ArrayExpr{Pos: pos, Elements: elements}
}

func (p *Parser) parseObject() ast.ExprNode {
	pos := p.current.Pos
	p.advance() // consume {
	p.skipLayout()

	fields := make([]ast.ObjectField, 0, 4)
	for p.current.Type != lexer.TokenRBrace && p.current.Type != lexer.TokenEOF {
		// Object keys are normally identifiers, but reserved keywords
		// like `and`, `or`, `not`, `query`, `match` are routinely used
		// as keys in query DSLs and HTTP-style payload shapes. Accept
		// any keyword token whose Val is the literal keyword text — the
		// parser stores the text and the AST treats it as a plain
		// string key.
		var keyTok lexer.Token
		switch {
		case isObjectKeyToken(p.current.Type):
			keyTok = p.advance()
		case p.current.Type == lexer.TokenString:
			// JSON-style quoted key, e.g. {"=": sop_id} or {"sort_order": 1}.
			keyTok = p.advance()
		default:
			keyTok = p.expect(lexer.TokenIdent)
		}
		p.expect(lexer.TokenColon)
		p.skipLayout()
		value := p.parseExpr()
		fields = append(fields, ast.ObjectField{Key: keyTok.Val, Value: value})
		p.skipLayout()
		if p.current.Type == lexer.TokenComma {
			p.advance()
			p.skipLayout()
		} else {
			break
		}
	}
	p.expect(lexer.TokenRBrace)
	return &ast.ObjectExpr{Pos: pos, Fields: fields}
}

// isObjectKeyToken reports whether a token type can serve as an object
// literal key. Identifiers always qualify; reserved keywords do too —
// callers frequently need keys named "and", "or", "not", "query" when
// building query DSL or other structured payloads.
func isObjectKeyToken(t lexer.TokenType) bool {
	switch t {
	case lexer.TokenIdent,
		lexer.TokenAnd,
		lexer.TokenOr,
		lexer.TokenNot,
		lexer.TokenIf,
		lexer.TokenThen,
		lexer.TokenElse,
		lexer.TokenMatch,
		lexer.TokenWhen,
		lexer.TokenTry,
		lexer.TokenCatch,
		lexer.TokenQuery,
		lexer.TokenFor,
		lexer.TokenIn,
		lexer.TokenRaise,
		lexer.TokenUse,
		lexer.TokenRecord,
		lexer.TokenTypeKw,
		lexer.TokenLet,
		lexer.TokenReturn,
		lexer.TokenTrue,
		lexer.TokenFalse,
		lexer.TokenNull,
		lexer.TokenFn:
		return true
	}
	return false
}

// --- If Expression ---

func (p *Parser) parseIf() ast.ExprNode {
	ifTok := p.advance() // consume 'if'
	p.skipNewlines()
	condition := p.parseExpr()

	p.skipNewlines()
	p.expect(lexer.TokenThen)
	p.skipNewlines()
	thenExpr := p.parseExpr()

	var elseExpr ast.ExprNode
	p.skipNewlines()
	if p.current.Type == lexer.TokenElse {
		p.advance()
		p.skipNewlines()
		// else if chains
		if p.current.Type == lexer.TokenIf {
			elseExpr = p.parseIf()
		} else {
			elseExpr = p.parseExpr()
		}
	}

	return &ast.IfExpr{
		Pos: ifTok.Pos, Condition: condition, Then: thenExpr, Else: elseExpr,
	}
}

// --- Match Expression ---

func (p *Parser) parseMatch() ast.ExprNode {
	matchTok := p.advance() // consume 'match'
	subject := p.parseExpr()

	p.expect(lexer.TokenColon)
	p.skipNewlines()

	arms := make([]ast.MatchArm, 0, 4)

	// Match arms: indented block or inline
	hasIndent := false
	if p.current.Type == lexer.TokenIndent {
		p.advance()
		hasIndent = true
	}

	for p.current.Type == lexer.TokenWhen {
		arm := p.parseMatchArm()
		arms = append(arms, arm)
		p.skipNewlines()
	}

	if hasIndent && p.current.Type == lexer.TokenDedent {
		p.advance()
	}

	return &ast.MatchExpr{Pos: matchTok.Pos, Subject: subject, Arms: arms}
}

func (p *Parser) parseMatchArm() ast.MatchArm {
	pos := p.current.Pos
	p.expect(lexer.TokenWhen)

	pattern := p.parsePattern()

	p.expect(lexer.TokenArrow)
	p.skipNewlines()
	body := p.parseExpr()

	return ast.MatchArm{Pos: pos, Pattern: pattern, Body: body}
}

func (p *Parser) parsePattern() ast.PatternNode {
	pos := p.current.Pos

	// Wildcard: _
	if p.current.Type == lexer.TokenIdent && p.current.Val == "_" {
		p.advance()
		return &ast.WildcardPattern{Pos: pos}
	}

	// Comparison pattern: < 0, >= 100, == "OK"
	if p.match(lexer.TokenLt, lexer.TokenGt, lexer.TokenLe, lexer.TokenGe, lexer.TokenEq, lexer.TokenNe) {
		op := p.advance()
		value := p.parsePrimary()
		return &ast.ComparisonPattern{Pos: pos, Op: op.Val, Value: value}
	}

	// Literal or range pattern
	left := p.parsePrimary()

	// Check for range: 1..10
	if p.current.Type == lexer.TokenDotDot {
		p.advance()
		right := p.parsePrimary()
		return &ast.RangePattern{Pos: pos, Low: left, High: right}
	}

	// Simple literal pattern
	if lit, ok := left.(*ast.LiteralExpr); ok {
		return &ast.LiteralPattern{Pos: pos, Value: lit.Value}
	}

	// Fallback: wrap expression as comparison with ==
	return &ast.ComparisonPattern{Pos: pos, Op: "==", Value: left}
}

// --- Try Expression ---

func (p *Parser) parseTry() ast.ExprNode {
	tryTok := p.advance() // consume 'try'
	expr := p.parseExpr()

	var defaultExpr ast.ExprNode
	if p.current.Type == lexer.TokenCatch {
		p.advance()
		defaultExpr = p.parseExpr()
	}

	return &ast.TryExpr{Pos: tryTok.Pos, Expr: expr, Default: defaultExpr}
}

// --- For Expression ---
// for item in items: body
// for item, idx in items: body

func (p *Parser) parseFor() ast.ExprNode {
	forTok := p.advance() // consume 'for'
	p.skipNewlines()

	varTok := p.expect(lexer.TokenIdent)
	variable := varTok.Val
	index := ""

	// Optional index variable: for item, idx in ...
	if p.current.Type == lexer.TokenComma {
		p.advance()
		idxTok := p.expect(lexer.TokenIdent)
		index = idxTok.Val
	}

	p.expect(lexer.TokenIn)
	p.skipNewlines()
	iterable := p.parseExpr()

	p.expect(lexer.TokenColon)
	p.skipNewlines()

	// Body can be a block or inline expression
	var body ast.ExprNode
	if p.current.Type == lexer.TokenIndent {
		stmts := p.parseBlock()
		if len(stmts) == 1 {
			if es, ok := stmts[0].(*ast.ExprStmt); ok {
				body = es.Expr
			}
		}
		if body == nil {
			// Multiple statements — wrap last as the result
			if len(stmts) > 0 {
				if es, ok := stmts[len(stmts)-1].(*ast.ExprStmt); ok {
					body = es.Expr
				}
			}
		}
		if body == nil {
			body = &ast.LiteralExpr{Pos: forTok.Pos, Value: nil, Type: "null"}
		}
	} else {
		body = p.parseExpr()
	}

	return &ast.ForExpr{
		Pos: forTok.Pos, Variable: variable, Index: index,
		Iterable: iterable, Body: body,
	}
}

// --- Raise Expression ---

func (p *Parser) parseRaise() ast.ExprNode {
	raiseTok := p.advance() // consume 'raise'
	p.skipNewlines()
	message := p.parseExpr()
	return &ast.RaiseExpr{Pos: raiseTok.Pos, Message: message}
}

// --- Query Expression ---

func (p *Parser) parseQuery() ast.ExprNode {
	pos := p.current.Pos
	p.advance() // consume 'query'
	p.expect(lexer.TokenLParen)
	dataset := p.parseExpr()
	p.expect(lexer.TokenRParen)

	return &ast.QueryExpr{Pos: pos, Dataset: dataset}
}

// --- Argument Lists ---

func (p *Parser) parseArgList() []ast.ExprNode {
	args := make([]ast.ExprNode, 0, 4)
	p.skipLayout()
	if p.current.Type == lexer.TokenRParen {
		return args
	}

	for {
		p.skipLayout()
		arg := p.parseExpr()
		args = append(args, arg)
		p.skipLayout()
		if p.current.Type != lexer.TokenComma {
			break
		}
		p.advance() // consume comma
	}
	return args
}
