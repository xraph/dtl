package ast

// Position tracks source location for error reporting.
type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// --- Top-Level ---

// FnAST is the root AST node for a DTL function definition.
type FnAST struct {
	Pos        Position
	Name       string
	Params     []ParamNode
	ReturnType TypeNode
	Body       []StmtNode    // one or more statements
	IsOneLiner bool          // true for => syntax
	TypeDefs   []TypeDefStmt // local type aliases defined before fn
}

// TypeDefStmt represents a local type alias: type Name = record { ... }
type TypeDefStmt struct {
	Pos  Position
	Name string   // the alias name (e.g. "Person")
	Type TypeNode // the record type definition
}

func (s *TypeDefStmt) stmtNode()         {}
func (s *TypeDefStmt) StmtPos() Position { return s.Pos }

// ParamNode describes a single function parameter.
type ParamNode struct {
	Pos      Position
	Name     string
	Type     TypeNode
	Default  ExprNode // nil if no default
	Variadic bool     // true for ...name syntax
}

// TypeNode represents a type annotation: "float", "string[]", "record { ... }", etc.
type TypeNode struct {
	Pos     Position
	Name    string        // "float", "string", "object", "record", named type, etc.
	IsArray bool          // true if declared as e.g. "float[]"
	Fields  []RecordField // non-nil only when Name == "record" (inline record definition)
}

// RecordField describes a single field in a record type.
type RecordField struct {
	Pos      Position
	Name     string
	Type     TypeNode // recursive — supports nested records
	Optional bool     // true if the field is optional (name?: type)
}

// --- Statements ---

// StmtNode is the interface for all statement nodes.
type StmtNode interface {
	stmtNode()
	StmtPos() Position
}

// LetStmt represents an immutable variable binding: let x = expr
type LetStmt struct {
	Pos   Position
	Name  string
	Value ExprNode
}

func (s *LetStmt) stmtNode()         {}
func (s *LetStmt) StmtPos() Position { return s.Pos }

// ReturnStmt represents an explicit return: return expr
type ReturnStmt struct {
	Pos   Position
	Value ExprNode
}

func (s *ReturnStmt) stmtNode()         {}
func (s *ReturnStmt) StmtPos() Position { return s.Pos }

// ExprStmt wraps an expression used as a statement (typically the last
// expression in a block acts as the implicit return value).
type ExprStmt struct {
	Pos  Position
	Expr ExprNode
}

func (s *ExprStmt) stmtNode()         {}
func (s *ExprStmt) StmtPos() Position { return s.Pos }

// --- Expressions ---

// ExprNode is the interface for all expression nodes.
type ExprNode interface {
	exprNode()
	ExprPos() Position
}

// LiteralExpr represents a literal value: 42, 3.14, "hello", true, null
type LiteralExpr struct {
	Pos   Position
	Value any    // Go-native value: int64, float64, string, bool, nil
	Type  string // "int", "float", "string", "bool", "null"
}

func (e *LiteralExpr) exprNode()         {}
func (e *LiteralExpr) ExprPos() Position { return e.Pos }

// IdentExpr references a variable by name.
type IdentExpr struct {
	Pos  Position
	Name string
}

func (e *IdentExpr) exprNode()         {}
func (e *IdentExpr) ExprPos() Position { return e.Pos }

// BinaryExpr represents a binary operation: a + b, x > 0, a && b, s ++ t
type BinaryExpr struct {
	Pos   Position
	Op    string // "+", "-", "*", "/", "%", "==", "!=", ">", "<", ">=", "<=", "&&", "||", "++"
	Left  ExprNode
	Right ExprNode
}

func (e *BinaryExpr) exprNode()         {}
func (e *BinaryExpr) ExprPos() Position { return e.Pos }

// UnaryExpr represents a unary operation: -x, !flag, not flag
type UnaryExpr struct {
	Pos     Position
	Op      string // "-", "!", "not"
	Operand ExprNode
}

func (e *UnaryExpr) exprNode()         {}
func (e *UnaryExpr) ExprPos() Position { return e.Pos }

// IfExpr represents a conditional expression: if cond then a else b
// Else can be another IfExpr for else-if chains.
type IfExpr struct {
	Pos       Position
	Condition ExprNode
	Then      ExprNode
	Else      ExprNode // nil if no else branch
}

func (e *IfExpr) exprNode()         {}
func (e *IfExpr) ExprPos() Position { return e.Pos }

// MatchExpr represents pattern matching: match value: when ...
type MatchExpr struct {
	Pos     Position
	Subject ExprNode
	Arms    []MatchArm
}

func (e *MatchExpr) exprNode()         {}
func (e *MatchExpr) ExprPos() Position { return e.Pos }

// MatchArm pairs a pattern with a result expression.
type MatchArm struct {
	Pos     Position
	Pattern PatternNode
	Body    ExprNode
}

// PipeExpr represents a pipe chain: expr | func(args)
// The input is implicitly prepended to the function's argument list.
type PipeExpr struct {
	Pos      Position
	Input    ExprNode
	Function string // function name (may include namespace)
	Args     []ExprNode
}

func (e *PipeExpr) exprNode()         {}
func (e *PipeExpr) ExprPos() Position { return e.Pos }

// FnCallExpr represents a function call: func_name(args)
type FnCallExpr struct {
	Pos  Position
	Name string // can be namespaced: "shared::analytics::anomaly"
	Args []ExprNode
}

func (e *FnCallExpr) exprNode()         {}
func (e *FnCallExpr) ExprPos() Position { return e.Pos }

// LambdaExpr represents an anonymous function: (x, y) => x + y
// Also used for desugared pipe shorthand: > 0 becomes (x) => x > 0
type LambdaExpr struct {
	Pos    Position
	Params []string
	Body   ExprNode
}

func (e *LambdaExpr) exprNode()         {}
func (e *LambdaExpr) ExprPos() Position { return e.Pos }

// ObjectExpr constructs an object literal: { key: value, ... }
// Fields is an ordered slice to preserve insertion order.
type ObjectExpr struct {
	Pos    Position
	Fields []ObjectField
}

// ObjectField is a single key-value pair in an object literal.
type ObjectField struct {
	Key   string
	Value ExprNode
}

func (e *ObjectExpr) exprNode()         {}
func (e *ObjectExpr) ExprPos() Position { return e.Pos }

// ArrayExpr constructs an array literal: [1, 2, 3]
type ArrayExpr struct {
	Pos      Position
	Elements []ExprNode
}

func (e *ArrayExpr) exprNode()         {}
func (e *ArrayExpr) ExprPos() Position { return e.Pos }

// IndexExpr accesses an element by index: arr[0], map["key"]
type IndexExpr struct {
	Pos    Position
	Object ExprNode
	Index  ExprNode
}

func (e *IndexExpr) exprNode()         {}
func (e *IndexExpr) ExprPos() Position { return e.Pos }

// FieldAccessExpr accesses a field on an object: obj.field
type FieldAccessExpr struct {
	Pos      Position
	Object   ExprNode
	Field    string
	Optional bool // true for ?. (optional chaining)
}

func (e *FieldAccessExpr) exprNode()         {}
func (e *FieldAccessExpr) ExprPos() Position { return e.Pos }

// TryExpr handles errors gracefully: try expr catch default
type TryExpr struct {
	Pos     Position
	Expr    ExprNode
	Default ExprNode
}

func (e *TryExpr) exprNode()         {}
func (e *TryExpr) ExprPos() Position { return e.Pos }

// CoalesceExpr provides null coalescing: a ?? b
type CoalesceExpr struct {
	Pos   Position
	Left  ExprNode
	Right ExprNode
}

func (e *CoalesceExpr) exprNode()         {}
func (e *CoalesceExpr) ExprPos() Position { return e.Pos }

// InterpolatedStringExpr represents a string with embedded expressions:
// "hello {name}, temp is {temp | round(1)}"
// Parts alternate between string literals and evaluated expressions.
type InterpolatedStringExpr struct {
	Pos   Position
	Parts []ExprNode // LiteralExpr (string parts) interspersed with expression nodes
}

func (e *InterpolatedStringExpr) exprNode()         {}
func (e *InterpolatedStringExpr) ExprPos() Position { return e.Pos }

// QueryExpr represents a dataset query.
// Two spellings parse to this same node:
//   - the `query` keyword form: query("dataset_name")
//   - the namespaced sugar:    dataset::query("dataset_name")
//
// The chain is populated when the query is piped into operators
// (where, select, etc.).
type QueryExpr struct {
	Pos     Position
	Dataset ExprNode   // expression that evaluates to dataset name
	Chain   []PipeExpr // downstream pipe operations (where, select, etc.)
}

func (e *QueryExpr) exprNode()         {}
func (e *QueryExpr) ExprPos() Position { return e.Pos }

// ForExpr represents a for-in expression that maps over a collection:
// for item in items: item.price * item.quantity
// for item, idx in items: {index: idx, value: item}
// Returns an array. Desugars semantically to map().
type ForExpr struct {
	Pos      Position
	Variable string   // loop variable name
	Index    string   // optional index variable (empty if not used)
	Iterable ExprNode // expression yielding an array
	Body     ExprNode // body evaluated per iteration
}

func (e *ForExpr) exprNode()         {}
func (e *ForExpr) ExprPos() Position { return e.Pos }

// RaiseExpr represents a user-defined error:
// raise "Temperature exceeds limit"
type RaiseExpr struct {
	Pos     Position
	Message ExprNode // expression that evaluates to error message
}

func (e *RaiseExpr) exprNode()         {}
func (e *RaiseExpr) ExprPos() Position { return e.Pos }

// InExpr represents a membership test:
// status in ["active", "pending"]
type InExpr struct {
	Pos        Position
	Value      ExprNode // left-hand value to test
	Collection ExprNode // right-hand array/object to check membership in
}

func (e *InExpr) exprNode()         {}
func (e *InExpr) ExprPos() Position { return e.Pos }

// UseStmt imports a namespace shortcut:
// use maintenance  → makes app:maintenance:: functions callable without prefix
type UseStmt struct {
	Pos       Position
	Namespace string // the namespace to import (e.g., "maintenance")
}

func (s *UseStmt) stmtNode()         {}
func (s *UseStmt) StmtPos() Position { return s.Pos }

// --- Patterns (used in match arms) ---

// PatternNode is the interface for match-arm patterns.
type PatternNode interface {
	patternNode()
	PatternPos() Position
}

// LiteralPattern matches a single literal value: when "OK", when 42
type LiteralPattern struct {
	Pos   Position
	Value any // Go-native literal
}

func (p *LiteralPattern) patternNode()         {}
func (p *LiteralPattern) PatternPos() Position { return p.Pos }

// ComparisonPattern matches a comparison: when < 0, when >= 100
type ComparisonPattern struct {
	Pos   Position
	Op    string   // "<", ">", "<=", ">=", "==", "!="
	Value ExprNode // the operand to compare against
}

func (p *ComparisonPattern) patternNode()         {}
func (p *ComparisonPattern) PatternPos() Position { return p.Pos }

// RangePattern matches an inclusive range: when 1..10
type RangePattern struct {
	Pos  Position
	Low  ExprNode
	High ExprNode
}

func (p *RangePattern) patternNode()         {}
func (p *RangePattern) PatternPos() Position { return p.Pos }

// WildcardPattern matches anything: when _
type WildcardPattern struct {
	Pos Position
}

func (p *WildcardPattern) patternNode()         {}
func (p *WildcardPattern) PatternPos() Position { return p.Pos }
