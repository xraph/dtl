package compiler

import (
	"fmt"
	"strings"

	"github.com/xraph/dtl/ast"
)

// FunctionResolver allows the compiler to check whether a function exists
// without depending on the registry package.
type FunctionResolver interface {
	ResolveFunction(name string) (paramCount int, exists bool)
}

// FunctionLister extends FunctionResolver with the ability to list all known
// function names, used for "did you mean?" suggestions.
type FunctionLister interface {
	FunctionResolver
	ListFunctionNames() []string
}

// CompileError is a single validation error with source position.
type CompileError struct {
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Code    string `json:"code"` // "undefined_var", "unknown_function", "type_mismatch", etc.
	Message string `json:"message"`
}

func (e CompileError) Error() string {
	return fmt.Sprintf("line %d, col %d: [%s] %s", e.Line, e.Column, e.Code, e.Message)
}

// CompileResult holds validated AST and resolved dependencies.
type CompileResult struct {
	AST          *ast.FnAST
	Dependencies []string // fully-qualified names of called functions
	Errors       []CompileError
}

// HasErrors returns true if compilation produced errors.
func (r *CompileResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// Compiler validates an AST and resolves dependencies.
type Compiler struct {
	resolver FunctionResolver
}

// New creates a compiler with the given function resolver.
func New(resolver FunctionResolver) *Compiler {
	return &Compiler{resolver: resolver}
}

// Compile validates the AST and returns a compilation result.
func (c *Compiler) Compile(fnAST *ast.FnAST) *CompileResult {
	result := &CompileResult{AST: fnAST}

	typeDefs := make(map[string]ast.TypeNode, len(fnAST.TypeDefs))
	for _, td := range fnAST.TypeDefs {
		typeDefs[td.Name] = td.Type
	}

	ctx := &compileCtx{
		result:   result,
		resolver: c.resolver,
		scope:    newScope(nil),
		typeDefs: typeDefs,
	}

	// Register parameters in scope with their declared types
	for _, param := range fnAST.Params {
		ctx.scope.defineTyped(param.Name, ctx.normalizeParamType(param.Type))
	}

	// Validate body statements
	for _, stmt := range fnAST.Body {
		ctx.checkStmt(stmt)
	}

	// Deduplicate dependencies
	result.Dependencies = uniqueStrings(result.Dependencies)
	return result
}

// --- Internal compilation context ---

type compileCtx struct {
	result   *CompileResult
	resolver FunctionResolver
	scope    *scope
	typeDefs map[string]ast.TypeNode

	// uses holds the namespaces declared by `use` statements, in the order
	// they appear. A bare call that names no local or builtin function is
	// resolved against them, mirroring what the executor does at runtime.
	// Order matters and so does position: the executor builds its namespace
	// list as statements execute, so a call placed above its `use` does not
	// resolve there either.
	uses []string
}

func (ctx *compileCtx) addError(pos ast.Position, code, message string) {
	ctx.result.Errors = append(ctx.result.Errors, CompileError{
		Line: pos.Line, Column: pos.Column, Code: code, Message: message,
	})
}

func (ctx *compileCtx) addDep(name string) {
	ctx.result.Dependencies = append(ctx.result.Dependencies, name)
}

// resolveType looks up a named type alias from local type definitions.
func (ctx *compileCtx) resolveType(name string) (ast.TypeNode, bool) {
	if td, ok := ctx.typeDefs[name]; ok {
		return td, true
	}
	return ast.TypeNode{}, false
}

// normalizeParamType resolves a parameter's TypeNode to an internal type
// category, handling both built-in types and named type aliases (e.g. a
// typedef that resolves to a record).
func (ctx *compileCtx) normalizeParamType(t ast.TypeNode) string {
	norm := normalizeType(t.Name)
	if norm != "" {
		return norm
	}
	// Try resolving as a named typedef
	if resolved, ok := ctx.resolveType(t.Name); ok {
		return normalizeType(resolved.Name)
	}
	return ""
}

// --- Scope for variable resolution ---

type scope struct {
	parent   *scope
	vars     map[string]bool
	varTypes map[string]string // tracked types: "number", "string", "bool", "array", "object", ""
}

func newScope(parent *scope) *scope {
	return &scope{parent: parent, vars: make(map[string]bool), varTypes: make(map[string]string)}
}

func (s *scope) define(name string) {
	s.vars[name] = true
}

func (s *scope) defineTyped(name, typ string) {
	s.vars[name] = true
	if typ != "" {
		s.varTypes[name] = typ
	}
}

// isDefinedLocal checks if the name is defined in this scope only (not parents).
func (s *scope) isDefinedLocal(name string) bool {
	return s.vars[name]
}

func (s *scope) isDefined(name string) bool {
	if s.vars[name] {
		return true
	}
	if s.parent != nil {
		return s.parent.isDefined(name)
	}
	return false
}

func (s *scope) typeOf(name string) string {
	if t, ok := s.varTypes[name]; ok {
		return t
	}
	if s.parent != nil {
		return s.parent.typeOf(name)
	}
	return ""
}

// child creates a nested scope inheriting from this one.
func (s *scope) child() *scope {
	return newScope(s)
}

// allDefined collects all variable names in scope (including parents).
func (s *scope) allDefined() []string {
	seen := make(map[string]bool)
	for cur := s; cur != nil; cur = cur.parent {
		for name := range cur.vars {
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	return names
}

// --- Statement Checking ---

func (ctx *compileCtx) checkStmt(stmt ast.StmtNode) {
	switch s := stmt.(type) {
	case *ast.LetStmt:
		ctx.checkExpr(s.Value)
		if ctx.scope.isDefinedLocal(s.Name) {
			ctx.addError(s.Pos, "duplicate_variable",
				fmt.Sprintf("variable %q is already declared in this scope", s.Name))
		}
		ctx.scope.defineTyped(s.Name, ctx.inferType(s.Value))
	case *ast.ReturnStmt:
		ctx.checkExpr(s.Value)
	case *ast.ExprStmt:
		ctx.checkExpr(s.Expr)
	case *ast.UseStmt:
		// Record the namespace so resolveFn can reach the functions it makes
		// visible. Without this the compiler rejects every cross-pack call at
		// install time, even though the executor resolves it happily.
		ctx.uses = append(ctx.uses, s.Namespace)
	}
}

// --- Expression Checking ---

func (ctx *compileCtx) checkExpr(expr ast.ExprNode) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.LiteralExpr:
		// Always valid

	case *ast.IdentExpr:
		// $ prefixed identifiers are pipeline variables, always valid
		if len(e.Name) > 0 && e.Name[0] == '$' {
			return
		}
		if !ctx.scope.isDefined(e.Name) {
			msg := fmt.Sprintf("variable %q is not defined", e.Name)
			if suggestion := ctx.suggestVariable(e.Name); suggestion != "" {
				msg += fmt.Sprintf(" — did you mean %q?", suggestion)
			}
			ctx.addError(e.Pos, "undefined_var", msg)
		}

	case *ast.BinaryExpr:
		ctx.checkExpr(e.Left)
		ctx.checkExpr(e.Right)
		ctx.checkBinaryTypes(e)

	case *ast.UnaryExpr:
		ctx.checkExpr(e.Operand)

	case *ast.IfExpr:
		ctx.checkExpr(e.Condition)
		ctx.checkExpr(e.Then)
		if e.Else != nil {
			ctx.checkExpr(e.Else)
		}

	case *ast.MatchExpr:
		ctx.checkExpr(e.Subject)
		hasWildcard := false
		for _, arm := range e.Arms {
			ctx.checkPattern(arm.Pattern)
			ctx.checkExpr(arm.Body)
			if _, ok := arm.Pattern.(*ast.WildcardPattern); ok {
				hasWildcard = true
			}
		}
		if !hasWildcard && len(e.Arms) > 0 {
			ctx.addError(e.Pos, "match_not_exhaustive",
				"match expression has no wildcard (_) arm; may not cover all cases")
		}

	case *ast.PipeExpr:
		ctx.checkExpr(e.Input)
		if e.Function != "" {
			ctx.resolveFn(e.Pos, e.Function)
		}
		for _, arg := range e.Args {
			ctx.checkExpr(arg)
		}

	case *ast.FnCallExpr:
		ctx.resolveFn(e.Pos, e.Name)
		for _, arg := range e.Args {
			ctx.checkExpr(arg)
		}

	case *ast.LambdaExpr:
		childScope := ctx.scope.child()
		for _, param := range e.Params {
			childScope.define(param)
		}
		childCtx := &compileCtx{
			result: ctx.result, resolver: ctx.resolver, scope: childScope, typeDefs: ctx.typeDefs,
		}
		childCtx.checkExpr(e.Body)

	case *ast.ObjectExpr:
		for _, field := range e.Fields {
			ctx.checkExpr(field.Value)
		}

	case *ast.ArrayExpr:
		for _, elem := range e.Elements {
			ctx.checkExpr(elem)
		}

	case *ast.IndexExpr:
		ctx.checkExpr(e.Object)
		ctx.checkExpr(e.Index)

	case *ast.FieldAccessExpr:
		ctx.checkExpr(e.Object)

	case *ast.TryExpr:
		ctx.checkExpr(e.Expr)
		if e.Default != nil {
			ctx.checkExpr(e.Default)
		}

	case *ast.CoalesceExpr:
		ctx.checkExpr(e.Left)
		ctx.checkExpr(e.Right)

	case *ast.InterpolatedStringExpr:
		for _, part := range e.Parts {
			ctx.checkExpr(part)
		}

	case *ast.QueryExpr:
		ctx.checkExpr(e.Dataset)
		for i := range e.Chain {
			ctx.checkExpr(e.Chain[i].Input)
			for _, arg := range e.Chain[i].Args {
				ctx.checkExpr(arg)
			}
		}

	case *ast.ForExpr:
		ctx.checkExpr(e.Iterable)
		childScope := ctx.scope.child()
		childScope.define(e.Variable)
		if e.Index != "" {
			childScope.define(e.Index)
		}
		childCtx := &compileCtx{
			result: ctx.result, resolver: ctx.resolver, scope: childScope, typeDefs: ctx.typeDefs,
		}
		childCtx.checkExpr(e.Body)

	case *ast.RaiseExpr:
		ctx.checkExpr(e.Message)

	case *ast.InExpr:
		ctx.checkExpr(e.Value)
		ctx.checkExpr(e.Collection)
	}
}

// --- Binary Type Checking ---

// checkBinaryTypes flags type mismatches in binary expressions.
func (ctx *compileCtx) checkBinaryTypes(e *ast.BinaryExpr) {
	leftType := ctx.inferType(e.Left)
	rightType := ctx.inferType(e.Right)

	switch e.Op {
	case "-", "*", "/", "%":
		if leftType == "string" {
			ctx.addError(e.Pos, "type_mismatch",
				fmt.Sprintf("operator %q requires numeric operands, got string on the left", e.Op))
		}
		if rightType == "string" {
			ctx.addError(e.Pos, "type_mismatch",
				fmt.Sprintf("operator %q requires numeric operands, got string on the right", e.Op))
		}
	case "+":
		if (leftType == "number" && rightType == "string") ||
			(leftType == "string" && rightType == "number") {
			ctx.addError(e.Pos, "type_mismatch",
				"mixing number and string with '+' — use TO_STRING() or TO_NUMBER() to make intent explicit")
		}
	}
}

// inferType attempts to statically determine the type of an expression.
// Returns "number", "string", "bool", "array", "object", or "" (unknown).
func (ctx *compileCtx) inferType(expr ast.ExprNode) string {
	if expr == nil {
		return ""
	}
	switch e := expr.(type) {
	case *ast.LiteralExpr:
		switch e.Type {
		case "int", "float":
			return "number"
		case "string":
			return "string"
		case "bool":
			return "bool"
		}

	case *ast.IdentExpr:
		return ctx.scope.typeOf(e.Name)

	case *ast.BinaryExpr:
		switch e.Op {
		case "-", "*", "/", "%":
			return "number"
		case "+":
			lt := ctx.inferType(e.Left)
			rt := ctx.inferType(e.Right)
			if lt == "string" || rt == "string" {
				return "string"
			}
			if lt == "number" || rt == "number" {
				return "number"
			}
		case "==", "!=", ">", "<", ">=", "<=", "&&", "||":
			return "bool"
		case "++":
			return "string"
		}

	case *ast.UnaryExpr:
		if e.Op == "!" || e.Op == "not" {
			return "bool"
		}
		return "number"

	case *ast.ArrayExpr:
		return "array"

	case *ast.ObjectExpr:
		return "object"

	case *ast.InterpolatedStringExpr:
		return "string"

	case *ast.IfExpr:
		// Infer from the then branch
		return ctx.inferType(e.Then)
	}

	return ""
}

// normalizeType maps DTL type annotations to our internal type categories.
func normalizeType(typ string) string {
	switch strings.ToLower(typ) {
	case "int", "float", "number":
		return "number"
	case "string":
		return "string"
	case "bool":
		return "bool"
	case "object", "record":
		return "object"
	}
	// Check if the type name resolves to a known typedef; if so, normalize the
	// resolved type. This handles named types like `type Sensor = record { ... }`.
	return ""
}

// --- Pattern Checking ---

func (ctx *compileCtx) checkPattern(pat ast.PatternNode) {
	switch p := pat.(type) {
	case *ast.LiteralPattern:
		// Always valid
	case *ast.ComparisonPattern:
		ctx.checkExpr(p.Value)
	case *ast.RangePattern:
		ctx.checkExpr(p.Low)
		ctx.checkExpr(p.High)
	case *ast.WildcardPattern:
		// Always valid
	}
}

// --- Function Resolution ---

// legacyBuiltinRedirect maps the previous flat-name spellings of platform
// builtins to their namespaced replacements. When a DTL function calls one of
// these legacy names the compiler rejects it with a deterministic migration
// hint (rather than a fuzzy "did you mean" suggestion). The `query` keyword
// is intentionally omitted — it remains valid as a grammar form alongside
// the dataset::query namespaced sugar.
var legacyBuiltinRedirect = map[string]string{
	"query_count":     "dataset::count",
	"schema_get":      "dataset::schema",
	"schema_columns":  "dataset::columns",
	"schema_validate": "dataset::validate",
	"http_get":        "http::get",
	"http_post":       "http::post",
	"pipeline_run":    "pipeline::run",
	"viz_transform":   "viz::transform",
	"agent_call":      "agent::call",
}

func (ctx *compileCtx) resolveFn(pos ast.Position, name string) {
	if replacement, deprecated := legacyBuiltinRedirect[name]; deprecated {
		ctx.addError(pos, "deprecated_builtin",
			fmt.Sprintf("function %q has been removed; use %q instead", name, replacement))
		ctx.addDep(name)
		return
	}

	// Skip resolution if no resolver is configured (e.g., validation-only mode)
	if ctx.resolver == nil {
		ctx.addDep(name)
		return
	}
	if _, exists := ctx.resolver.ResolveFunction(name); exists {
		ctx.addDep(name)
		return
	}

	// Not a bare name the registry knows. Try the namespaces the function
	// declared with `use`, in the same order and the same two forms the
	// executor tries at runtime (see evalCall's use-statement fallback). The
	// registry only ever holds the qualified key, so this is the only way a
	// cross-pack call can resolve — and matching the executor exactly is what
	// keeps "compiles" and "runs" from disagreeing.
	for _, ns := range ctx.uses {
		for _, qualified := range []string{"app:" + ns + "::" + name, ns + "::" + name} {
			if _, exists := ctx.resolver.ResolveFunction(qualified); exists {
				// Record the dependency the pack actually has. The bare name
				// exists in no registry, so tracking it would make the
				// cross-pack dependency invisible to install ordering.
				ctx.addDep(qualified)
				return
			}
		}
	}

	msg := fmt.Sprintf("function %q is not defined", name)
	if suggestion := ctx.suggestFunction(name); suggestion != "" {
		msg += fmt.Sprintf(" — did you mean %q?", suggestion)
	}
	ctx.addError(pos, "unknown_function", msg)
	ctx.addDep(name)
}

// suggestFunction returns the closest matching function name, or empty string.
func (ctx *compileCtx) suggestFunction(name string) string {
	lister, ok := ctx.resolver.(FunctionLister)
	if !ok {
		return ""
	}
	return closestMatch(name, lister.ListFunctionNames(), 3)
}

// suggestVariable returns the closest matching variable name, or empty string.
func (ctx *compileCtx) suggestVariable(name string) string {
	known := ctx.scope.allDefined()
	return closestMatch(name, known, 3)
}

// --- Helpers ---

// closestMatch returns the best match from candidates within maxDist edits.
// Returns empty string if no close match is found.
func closestMatch(target string, candidates []string, maxDist int) string {
	if len(candidates) == 0 {
		return ""
	}

	target = strings.ToLower(target)
	best := ""
	bestDist := maxDist + 1

	for _, c := range candidates {
		if strings.HasPrefix(c, "__") {
			continue // skip internal vars
		}
		d := levenshtein(target, strings.ToLower(c))
		if d < bestDist {
			bestDist = d
			best = c
		}
	}

	if bestDist <= maxDist {
		return best
	}

	// Also try prefix matching for partial names
	for _, c := range candidates {
		if strings.HasPrefix(c, "__") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(c), target) || strings.HasPrefix(target, strings.ToLower(c)) {
			return c
		}
	}
	return ""
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	la := len(a)
	lb := len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = minInt(minInt(curr[j-1]+1, prev[j]+1), prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func uniqueStrings(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	result := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
