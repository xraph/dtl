package executor

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/xraph/dtl/ast"
	"github.com/xraph/dtl/capability"
)

// Errors returned by the executor.
var (
	ErrTimeout  = errors.New("execution timeout exceeded")
	ErrMaxDepth = errors.New("maximum call depth exceeded")
	ErrDivZero  = errors.New("division by zero")
)

// BuiltinFunc is a Go-implemented function callable from DTL.
type BuiltinFunc struct {
	Name    string
	MinArgs int
	MaxArgs int // -1 for variadic
	Fn      func(args []any) (any, error)
	// CtxFn is an optional context-aware alternative to Fn.
	// When set, the executor passes the evaluation context so the function
	// can access platform services (schema, query, pipeline, etc.).
	// If CtxFn is set, Fn is ignored.
	CtxFn func(ctx context.Context, args []any) (any, error)
	// Doc is a one-line signature and description, surfaced by the language
	// server as hover text and completion detail. The conventional shape is
	// "name(args) -> type -- what it does". Host-registered builtins may set
	// it to document themselves; an empty Doc simply yields no hover text.
	Doc string
}

// CompiledFunction is a parsed+compiled user-defined function ready for execution.
type CompiledFunction struct {
	Name   string
	AST    *ast.FnAST
	Deps   []string
	Source string
}

// FunctionLookup resolves function names to compiled user functions.
type FunctionLookup interface {
	GetCompiled(ctx context.Context, name string) (*CompiledFunction, bool)
}

// DebugEntry represents a single debug/print output line.
type DebugEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Label     string    `json:"label,omitempty"`
	Values    []any     `json:"values"`
}

type debugBufferKey struct{}

// NewDebugContext returns a context with an attached debug buffer.
//
// Deprecated: ExecuteWithDebug collects the same entries without allocating a
// buffer and a context value on every call, whether or not the function ever
// calls debug(). This remains for callers driving the executor directly, and
// appendDebug still honours it.
func NewDebugContext(ctx context.Context) (context.Context, *[]DebugEntry) {
	buf := &[]DebugEntry{}
	return context.WithValue(ctx, debugBufferKey{}, buf), buf
}

// appendDebug records an entry on the execution's own sink.
//
// Only debug() and print() reach here, so the sink stays nil for the
// overwhelming majority of executions and costs nothing until first written.
// The context lookup is the fallback for callers still using NewDebugContext.
func (ec *evalCtx) appendDebug(entry DebugEntry) {
	if ec.debug != nil {
		*ec.debug = append(*ec.debug, entry)
		return
	}
	if buf, ok := ec.ctx.Value(debugBufferKey{}).(*[]DebugEntry); ok {
		*buf = append(*buf, entry)
	}
}

// ExecConfig holds execution limits.
//
// Both fields use zero to mean "unbounded", which is the right primitive at
// this layer but the wrong default for a host: unbounded depth ends in
// `fatal error: stack overflow`, which no recover() can catch. Callers that
// build an Executor directly own that choice. Anything constructed through
// registry.New gets registry.DefaultMaxCallDepth instead.
type ExecConfig struct {
	// Timeout bounds wall-clock time per execution. Zero means unbounded.
	Timeout time.Duration

	// MaxDepth bounds nested user-function calls. Zero means unbounded.
	MaxDepth int
}

// Executor evaluates compiled DTL AST nodes via tree-walking.
type Executor struct {
	// builtins is the base table. It may be shared with other executors — see
	// stdlib.Shared — so it is only ever read.
	builtins map[string]*BuiltinFunc

	// overrides holds builtins registered against this executor alone, and is
	// nil until one is. Keeping them separate is what lets the base table be
	// shared: a host registering "store::get" must not add it to every other
	// registry in the process.
	overrides map[string]*BuiltinFunc

	lookup FunctionLookup
	config ExecConfig
}

// New creates an executor with the given built-ins and function resolver.
//
// builtins is not copied and must not be mutated afterwards; use
// RegisterBuiltin to add to this executor without touching a shared table.
func New(builtins map[string]*BuiltinFunc, lookup FunctionLookup, config ExecConfig) *Executor {
	if builtins == nil {
		builtins = make(map[string]*BuiltinFunc)
	}
	return &Executor{builtins: builtins, lookup: lookup, config: config}
}

// RegisterBuiltin adds a builtin visible only to this executor, shadowing any
// same-named entry in the base table.
//
// Not safe against concurrent execution: call it during setup, before the
// executor is used. That was already the contract when the builtin table was
// written directly.
func (ex *Executor) RegisterBuiltin(name string, bf *BuiltinFunc) {
	if ex.overrides == nil {
		ex.overrides = make(map[string]*BuiltinFunc, 8)
	}
	ex.overrides[name] = bf
}

// lookupBuiltin resolves a name against this executor's own registrations
// first, then the shared base table. The nil check keeps the common case —
// no host builtins at all — down to a single map probe.
func (ex *Executor) lookupBuiltin(name string) (*BuiltinFunc, bool) {
	if ex.overrides != nil {
		if bf, ok := ex.overrides[name]; ok {
			return bf, true
		}
	}
	bf, ok := ex.builtins[name]
	return bf, ok
}

// Execute runs a compiled function with the given arguments.
func (ex *Executor) Execute(ctx context.Context, fn *CompiledFunction, args map[string]any) (any, error) {
	return ex.ExecuteInto(ctx, fn, args, nil)
}

// ExecuteWithDebug runs a compiled function and returns whatever debug() and
// print() emitted, without the per-call buffer and context value that
// NewDebugContext costs. The returned slice is nil unless something was
// emitted, so the common case allocates nothing for it.
func (ex *Executor) ExecuteWithDebug(ctx context.Context, fn *CompiledFunction, args map[string]any) (any, []DebugEntry, error) {
	var logs []DebugEntry
	val, err := ex.ExecuteInto(ctx, fn, args, &logs)
	return val, logs, err
}

// ExecuteInto runs a compiled function, appending any debug() or print()
// output to *logs. Pass nil to discard it.
//
// This exists so a caller that already has somewhere to put the entries can
// hand that address over — registry.Execute points it at the ExecuteResult it
// was going to allocate regardless. Owning the destination is what keeps the
// executor's own per-call state off the heap: a sink pointing into evalCtx
// would force evalCtx itself to escape, trading one allocation for another.
func (ex *Executor) ExecuteInto(ctx context.Context, fn *CompiledFunction, args map[string]any, logs *[]DebugEntry) (any, error) {
	env := newEnv(nil)

	ec := &evalCtx{
		ctx:   ctx,
		env:   env,
		depth: 0,
		start: time.Now(),
		debug: logs,
	}

	// Bind parameters from args map, applying defaults and validating record types.
	for _, param := range fn.AST.Params {
		if val, ok := args[param.Name]; ok {
			// Validate record-typed parameters at call boundary.
			if len(param.Type.Fields) > 0 {
				if err := validateRecordArg(param.Name, val, param.Type); err != nil {
					return nil, err
				}
			}
			env.set(param.Name, val)
		} else if param.Default != nil {
			// Defaults evaluate in the scope built so far, so an earlier
			// parameter is visible to a later one's default.
			defVal, err := ex.evalExpr(ec, param.Default)
			if err != nil {
				return nil, fmt.Errorf("evaluating default for %s: %w", param.Name, err)
			}
			env.set(param.Name, defVal)
		} else {
			// No arg provided and no default — bind to nil so the variable exists
			env.set(param.Name, nil)
		}
	}

	return ex.execBody(ec, fn.AST.Body)
}

// ExecuteExpr evaluates a standalone expression with the given variable bindings.
func (ex *Executor) ExecuteExpr(ctx context.Context, expr ast.ExprNode, vars map[string]any) (any, error) {
	env := newEnv(nil)
	for k, v := range vars {
		env.set(k, v)
	}
	ec := &evalCtx{ctx: ctx, env: env, depth: 0, start: time.Now()}
	return ex.evalExpr(ec, expr)
}

// --- Environment (lexical scope chain) ---

// envInline is how many bindings a scope holds without a second allocation.
//
// A map costs three allocations to reach one binding — the env, the hmap, and
// its first bucket — and every scope in a DTL program pays that, including the
// one-parameter functions that dominate real use. Scopes here are tiny and
// short-lived, so inline storage with a linear scan is both fewer allocations
// and fewer indirections than hashing. Four covers a function's parameters and
// a couple of lets; anything wider spills to the slices below and still costs
// only what a map would have.
const envInline = 4

type env struct {
	parent *env

	n     int
	names [envInline]string
	vals  [envInline]any

	// Spill storage, allocated only by a scope that exceeds envInline.
	moreNames []string
	moreVals  []any

	// reusable marks a scope that will be rebound and run again — a lambda's
	// per-iteration scope. Anything that outlives one iteration must snapshot
	// it rather than keep the pointer; see snapshot and evalLambda.
	reusable bool
}

func newEnv(parent *env) *env {
	return &env{parent: parent}
}

// reset rebinds an existing scope to a new parent and drops its bindings, so
// one allocation can serve every iteration of a loop.
func (e *env) reset(parent *env) {
	e.parent = parent
	// Clear rather than truncate: the arrays hold `any` and `string`, and
	// leaving stale entries would keep whatever they reference alive.
	for i := 0; i < e.n; i++ {
		e.names[i] = ""
		e.vals[i] = nil
	}
	e.n = 0
	for i := range e.moreNames {
		e.moreNames[i] = ""
		e.moreVals[i] = nil
	}
	e.moreNames = e.moreNames[:0]
	e.moreVals = e.moreVals[:0]
}

// snapshot returns a scope with the same bindings that will not be rebound.
//
// It copies the chain for as long as the scopes are reusable, because a
// closure capturing a scratch scope would otherwise observe the next
// iteration's values. Non-reusable ancestors are shared, not copied: they are
// stable for the closure's lifetime already.
func (e *env) snapshot() *env {
	if e == nil || !e.reusable {
		return e
	}
	cp := &env{
		parent: e.parent.snapshot(),
		n:      e.n,
		names:  e.names,
		vals:   e.vals,
	}
	if len(e.moreNames) > 0 {
		cp.moreNames = append([]string(nil), e.moreNames...)
		cp.moreVals = append([]any(nil), e.moreVals...)
	}
	return cp
}

// lookupLocal finds a binding in this scope only, without walking to parents.
func (e *env) lookupLocal(name string) (any, bool) {
	for i := 0; i < e.n; i++ {
		if e.names[i] == name {
			return e.vals[i], true
		}
	}
	for i, n := range e.moreNames {
		if n == name {
			return e.moreVals[i], true
		}
	}
	return nil, false
}

func (e *env) get(name string) (any, bool) {
	// Walk the chain iteratively; a deep scope chain would otherwise add a Go
	// frame per level on a path already bounded by MaxDepth.
	for s := e; s != nil; s = s.parent {
		if val, ok := s.lookupLocal(name); ok {
			return val, true
		}
	}
	return nil, false
}

// set binds name in this scope, replacing any binding already made here.
// Rebinding must overwrite rather than append, or a shadowed value could be
// found first by the linear scan.
func (e *env) set(name string, val any) {
	for i := 0; i < e.n; i++ {
		if e.names[i] == name {
			e.vals[i] = val
			return
		}
	}
	for i, n := range e.moreNames {
		if n == name {
			e.moreVals[i] = val
			return
		}
	}
	if e.n < envInline {
		e.names[e.n] = name
		e.vals[e.n] = val
		e.n++
		return
	}
	e.moreNames = append(e.moreNames, name)
	e.moreVals = append(e.moreVals, val)
}

// --- Evaluation context ---

type evalCtx struct {
	ctx   context.Context
	env   *env
	depth int
	start time.Time

	// debug points at the root execution's entry slice, shared by every nested
	// scope. Nil when no sink was requested; the slice itself stays nil until a
	// debug() or print() actually runs.
	debug *[]DebugEntry
}

func (ec *evalCtx) checkLimits(ex *Executor) error {
	if ex.config.Timeout > 0 && time.Since(ec.start) > ex.config.Timeout {
		return ErrTimeout
	}
	if ex.config.MaxDepth > 0 && ec.depth > ex.config.MaxDepth {
		return ErrMaxDepth
	}
	if err := ec.ctx.Err(); err != nil {
		return err
	}
	return nil
}

// --- Body Execution ---

func (ex *Executor) execBody(ec *evalCtx, stmts []ast.StmtNode) (any, error) {
	var result any
	for _, stmt := range stmts {
		val, err := ex.execStmt(ec, stmt)
		if err != nil {
			return nil, err
		}
		result = val
	}
	return result, nil
}

func (ex *Executor) execStmt(ec *evalCtx, stmt ast.StmtNode) (any, error) {
	if err := ec.checkLimits(ex); err != nil {
		return nil, err
	}

	switch s := stmt.(type) {
	case *ast.LetStmt:
		val, err := ex.evalExpr(ec, s.Value)
		if err != nil {
			return nil, err
		}
		ec.env.set(s.Name, val)
		return nil, nil

	case *ast.ReturnStmt:
		return ex.evalExpr(ec, s.Value)

	case *ast.ExprStmt:
		return ex.evalExpr(ec, s.Expr)

	case *ast.UseStmt:
		// Use statements register namespace shortcuts in the environment.
		// The registry handles resolution; here we just store the alias.
		uses, _ := ec.env.get("__uses__")
		usesSlice, _ := uses.([]any)
		usesSlice = append(usesSlice, s.Namespace)
		ec.env.set("__uses__", usesSlice)
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown statement type: %T", stmt)
	}
}

// --- Expression Evaluation ---

func (ex *Executor) evalExpr(ec *evalCtx, expr ast.ExprNode) (any, error) {
	if err := ec.checkLimits(ex); err != nil {
		return nil, err
	}

	switch e := expr.(type) {
	case *ast.LiteralExpr:
		return e.Value, nil

	case *ast.IdentExpr:
		return ex.evalIdent(ec, e)

	case *ast.BinaryExpr:
		return ex.evalBinary(ec, e)

	case *ast.UnaryExpr:
		return ex.evalUnary(ec, e)

	case *ast.IfExpr:
		return ex.evalIf(ec, e)

	case *ast.MatchExpr:
		return ex.evalMatch(ec, e)

	case *ast.PipeExpr:
		return ex.evalPipe(ec, e)

	case *ast.FnCallExpr:
		return ex.evalFnCall(ec, e)

	case *ast.LambdaExpr:
		return ex.evalLambda(ec, e)

	case *ast.ObjectExpr:
		return ex.evalObject(ec, e)

	case *ast.ArrayExpr:
		return ex.evalArray(ec, e)

	case *ast.IndexExpr:
		return ex.evalIndex(ec, e)

	case *ast.FieldAccessExpr:
		return ex.evalFieldAccess(ec, e)

	case *ast.TryExpr:
		return ex.evalTry(ec, e)

	case *ast.CoalesceExpr:
		return ex.evalCoalesce(ec, e)

	case *ast.InterpolatedStringExpr:
		return ex.evalInterpolatedString(ec, e)

	case *ast.QueryExpr:
		return ex.evalQuery(ec, e)

	case *ast.ForExpr:
		return ex.evalFor(ec, e)

	case *ast.RaiseExpr:
		return ex.evalRaise(ec, e)

	case *ast.InExpr:
		return ex.evalIn(ec, e)

	default:
		return nil, fmt.Errorf("unknown expression type: %T", expr)
	}
}

func (ex *Executor) evalIdent(ec *evalCtx, e *ast.IdentExpr) (any, error) {
	val, ok := ec.env.get(e.Name)
	if !ok {
		return nil, fmt.Errorf("undefined variable: %s", e.Name)
	}
	return val, nil
}

func (ex *Executor) evalBinary(ec *evalCtx, e *ast.BinaryExpr) (any, error) {
	// Short-circuit for logical operators
	if e.Op == "&&" {
		left, err := ex.evalExpr(ec, e.Left)
		if err != nil {
			return nil, err
		}
		if !toBool(left) {
			return false, nil
		}
		right, err := ex.evalExpr(ec, e.Right)
		if err != nil {
			return nil, err
		}
		return toBool(right), nil
	}
	if e.Op == "||" {
		left, err := ex.evalExpr(ec, e.Left)
		if err != nil {
			return nil, err
		}
		if toBool(left) {
			return true, nil
		}
		right, err := ex.evalExpr(ec, e.Right)
		if err != nil {
			return nil, err
		}
		return toBool(right), nil
	}

	left, err := ex.evalExpr(ec, e.Left)
	if err != nil {
		return nil, err
	}
	right, err := ex.evalExpr(ec, e.Right)
	if err != nil {
		return nil, err
	}

	// String concatenation (explicit ++ operator)
	if e.Op == "++" {
		return toString(left) + toString(right), nil
	}

	// The + operator also concatenates when either operand is a string.
	// This makes "hello " + "world" and "count: " + 42 work naturally.
	if e.Op == "+" {
		_, lStr := left.(string)
		_, rStr := right.(string)
		if lStr || rStr {
			return toString(left) + toString(right), nil
		}
	}

	// Arithmetic
	switch e.Op {
	case "+", "-", "*", "/", "%":
		return evalArith(e.Op, left, right)
	case "==":
		return equalValues(left, right), nil
	case "!=":
		return !equalValues(left, right), nil
	case "<", ">", "<=", ">=":
		return compareValues(e.Op, left, right)
	default:
		return nil, fmt.Errorf("unknown operator: %s", e.Op)
	}
}

func (ex *Executor) evalUnary(ec *evalCtx, e *ast.UnaryExpr) (any, error) {
	val, err := ex.evalExpr(ec, e.Operand)
	if err != nil {
		return nil, err
	}
	switch e.Op {
	case "-":
		return negateValue(val)
	case "!", "not":
		return !toBool(val), nil
	default:
		return nil, fmt.Errorf("unknown unary operator: %s", e.Op)
	}
}

func (ex *Executor) evalIf(ec *evalCtx, e *ast.IfExpr) (any, error) {
	cond, err := ex.evalExpr(ec, e.Condition)
	if err != nil {
		return nil, err
	}
	if toBool(cond) {
		return ex.evalExpr(ec, e.Then)
	}
	if e.Else != nil {
		return ex.evalExpr(ec, e.Else)
	}
	return nil, nil
}

func (ex *Executor) evalMatch(ec *evalCtx, e *ast.MatchExpr) (any, error) {
	subject, err := ex.evalExpr(ec, e.Subject)
	if err != nil {
		return nil, err
	}

	for _, arm := range e.Arms {
		matched, err := ex.matchPattern(ec, arm.Pattern, subject)
		if err != nil {
			return nil, err
		}
		if matched {
			return ex.evalExpr(ec, arm.Body)
		}
	}
	return nil, nil // no match
}

func (ex *Executor) matchPattern(ec *evalCtx, pat ast.PatternNode, subject any) (bool, error) {
	switch p := pat.(type) {
	case *ast.WildcardPattern:
		return true, nil

	case *ast.LiteralPattern:
		return equalValues(subject, p.Value), nil

	case *ast.ComparisonPattern:
		val, err := ex.evalExpr(ec, p.Value)
		if err != nil {
			return false, err
		}
		result, err := compareValues(p.Op, subject, val)
		if err != nil {
			return false, nil // type mismatch in comparison = no match
		}
		return result, nil

	case *ast.RangePattern:
		low, err := ex.evalExpr(ec, p.Low)
		if err != nil {
			return false, err
		}
		high, err := ex.evalExpr(ec, p.High)
		if err != nil {
			return false, err
		}
		geqLow, err := compareValues(">=", subject, low)
		if err != nil {
			return false, nil
		}
		leqHigh, err := compareValues("<=", subject, high)
		if err != nil {
			return false, nil
		}
		return geqLow && leqHigh, nil

	default:
		return false, fmt.Errorf("unknown pattern type: %T", pat)
	}
}

func (ex *Executor) evalQuery(ec *evalCtx, e *ast.QueryExpr) (any, error) {
	dataset, err := ex.evalExpr(ec, e.Dataset)
	if err != nil {
		return nil, err
	}

	result, err := ex.callFunction(ec, "dataset::query", []any{dataset})
	if err != nil {
		return nil, err
	}

	// Apply pipe chain if present.
	// Both spellings desugar here: `query("doc") | where(...)` and
	// `dataset::query("doc") | where(...)` produce the same QueryExpr.
	for _, pipe := range e.Chain {
		args := make([]any, 0, 1+len(pipe.Args))
		args = append(args, result)
		for _, argExpr := range pipe.Args {
			val, err := ex.evalExpr(ec, argExpr)
			if err != nil {
				return nil, err
			}
			args = append(args, val)
		}
		result, err = ex.callFunction(ec, pipe.Function, args)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (ex *Executor) evalPipe(ec *evalCtx, e *ast.PipeExpr) (any, error) {
	input, err := ex.evalExpr(ec, e.Input)
	if err != nil {
		return nil, err
	}

	// Evaluate additional args
	args := make([]any, 0, 1+len(e.Args))
	args = append(args, input) // pipe input is always the first argument
	for _, argExpr := range e.Args {
		val, err := ex.evalExpr(ec, argExpr)
		if err != nil {
			return nil, err
		}
		args = append(args, val)
	}

	// If function name is empty, e.Args[0] might be a lambda expression
	if e.Function == "" && len(e.Args) > 0 {
		return nil, fmt.Errorf("pipe without function name not supported")
	}

	return ex.callFunction(ec, e.Function, args)
}

func (ex *Executor) evalFnCall(ec *evalCtx, e *ast.FnCallExpr) (any, error) {
	args := make([]any, 0, len(e.Args))
	for _, argExpr := range e.Args {
		val, err := ex.evalExpr(ec, argExpr)
		if err != nil {
			return nil, err
		}
		args = append(args, val)
	}
	return ex.callFunction(ec, e.Name, args)
}

func (ex *Executor) callFunction(ec *evalCtx, name string, args []any) (any, error) {
	// Intercept debug/print calls for output capture
	upper := strings.ToUpper(name)
	if upper == "DEBUG" || upper == "PRINT" {
		entry := DebugEntry{
			Timestamp: time.Now(),
			Values:    args,
		}
		// If first arg is a string and there are more args, treat it as a label
		if upper == "DEBUG" && len(args) >= 2 {
			if label, ok := args[0].(string); ok {
				entry.Label = label
				entry.Values = args[1:]
			}
		}
		ec.appendDebug(entry)
		// Return last value so it's chainable
		if len(args) > 0 {
			return args[len(args)-1], nil
		}
		return nil, nil
	}

	// An explicitly imported name wins over an ambient builtin, the way an
	// import shadows a global in any other language.
	//
	// Without this, any pack function whose name happens to match a stdlib
	// builtin is simply unreachable: `use com.example.core.approvals` followed
	// by `sign(...)` resolved to the MATH sign function and failed with an
	// arity error, naming neither the pack nor the collision. `count`,
	// `find`, `first`, `merge` and `normalize` are all builtins too, so this
	// was a landmine under the whole cross-pack call mechanism rather than one
	// unlucky name.
	//
	// Only names a declared namespace actually provides are affected — an
	// unmatched name falls straight through to the builtin below.
	if v, err, ok := ex.resolveViaUses(ec, name, args); ok {
		return v, err
	}

	// Check built-ins first
	if bf, ok := ex.lookupBuiltin(name); ok {
		if bf.MinArgs >= 0 && len(args) < bf.MinArgs {
			return nil, fmt.Errorf("%s: expected at least %d arguments, got %d", name, bf.MinArgs, len(args))
		}
		if bf.MaxArgs >= 0 && len(args) > bf.MaxArgs {
			return nil, fmt.Errorf("%s: expected at most %d arguments, got %d", name, bf.MaxArgs, len(args))
		}
		if bf.CtxFn != nil {
			return bf.CtxFn(ec.ctx, args)
		}
		return bf.Fn(args)
	}

	// Check user-defined functions
	if ex.lookup != nil {
		if fn, ok := ex.lookup.GetCompiled(ec.ctx, name); ok {
			return ex.callUserFunction(ec, fn, args)
		}
	}

	return nil, fmt.Errorf("function %q is not defined", name)
}

// resolveViaUses looks name up against every namespace the function declared
// with `use`, in declaration order, trying both the app:{ns}::{name} form the
// function registry uses and the bare {ns}::{name} form of shared helpers.
//
// It runs BEFORE the builtin lookup so an explicitly imported name shadows an
// ambient one; see the call site for why that matters. The bool reports
// whether anything matched, so an unmatched name falls through untouched.
func (ex *Executor) resolveViaUses(ec *evalCtx, name string, args []any) (any, error, bool) {
	uses, ok := ec.env.get("__uses__")
	if !ok {
		return nil, nil, false
	}
	usesSlice, ok := uses.([]any)
	if !ok {
		return nil, nil, false
	}
	for _, ns := range usesSlice {
		nsStr := toString(ns)
		for _, qualified := range []string{"app:" + nsStr + "::" + name, nsStr + "::" + name} {
			if bf, found := ex.lookupBuiltin(qualified); found {
				if bf.CtxFn != nil {
					v, err := bf.CtxFn(ec.ctx, args)
					return v, err, true
				}
				v, err := bf.Fn(args)
				return v, err, true
			}
			if ex.lookup != nil {
				if fn, found := ex.lookup.GetCompiled(ec.ctx, qualified); found {
					v, err := ex.callUserFunction(ec, fn, args)
					return v, err, true
				}
			}
		}
	}
	return nil, nil, false
}

func (ex *Executor) callUserFunction(ec *evalCtx, fn *CompiledFunction, args []any) (any, error) {
	childEc := &evalCtx{
		// Re-scope authority as execution crosses into a user function. The
		// embedder decides what that means; with no interceptor installed this
		// is inert. A host that groups functions into packages typically
		// installs a guard here, confining each one's writes to its own grants.
		ctx:   capability.Enter(ec.ctx, fn.Name),
		env:   newEnv(nil), // user functions get a clean scope
		depth: ec.depth + 1,
		start: ec.start,
		debug: ec.debug, // a callee's debug() output belongs to the same run
	}

	if err := childEc.checkLimits(ex); err != nil {
		return nil, err
	}

	// Bind positional args to parameters
	for i, param := range fn.AST.Params {
		if i < len(args) {
			childEc.env.set(param.Name, args[i])
		} else if param.Default != nil {
			defVal, err := ex.evalExpr(childEc, param.Default)
			if err != nil {
				return nil, err
			}
			childEc.env.set(param.Name, defVal)
		}
	}

	return ex.execBody(childEc, fn.AST.Body)
}

func (ex *Executor) evalLambda(ec *evalCtx, e *ast.LambdaExpr) (any, error) {
	// Capture the current environment for closure
	// snapshot, not the scope itself: if this lambda was created inside another
	// lambda's body, ec.env is that lambda's per-iteration scratch and will be
	// rebound on its next iteration. Taking a copy here is what makes scratch
	// reuse safe, and it costs nothing outside that nested case.
	capturedEnv := ec.env.snapshot()
	return &lambdaClosure{
		params: e.Params, body: e.Body, env: capturedEnv, executor: ex,
		// Captured here rather than passed to Call, because the public
		// CallLambda entry point stdlib uses carries only a context.
		debug: ec.debug,
		depth: ec.depth,
		start: ec.start,
	}, nil
}

// lambdaClosure captures a lambda and its enclosing environment.
type lambdaClosure struct {
	params   []string
	body     ast.ExprNode
	env      *env
	executor *Executor
	debug    *[]DebugEntry

	// depth and start are the execution limits in force where this lambda was
	// written, carried on the closure because the callers that invoke it
	// cannot supply them: stdlib's higher-order functions are plain
	// func(args []any) with no access to the running evalCtx, so they passed
	// time.Now() and 0 — restarting the deadline and resetting the depth
	// counter on every element. Recursion routed through map or filter
	// therefore ran unbounded and crashed the process, and a timeout could
	// never fire inside a collection.
	depth int
	start time.Time

	// scratch is this closure's argument scope, reused across calls. filter,
	// map and reduce invoke the same closure once per element, and allocating
	// a scope each time made newEnv 87% of the collection workload's
	// allocations. inCall guards against reentrancy — a lambda whose body
	// reaches this same closure again must not rebind the scope it is using.
	scratch *env
	inCall  bool
}

// Call invokes a lambda closure with the given arguments.
//
// start and depth are ignored; the closure carries the limits in force where
// it was written. They remain in the signature because CallLambda is public
// and stdlib passes them, but honouring a caller-supplied depth of 0 is what
// let recursion through map and filter run unbounded.
func (lc *lambdaClosure) Call(ctx context.Context, args []any, _ time.Time, _ int) (any, error) {
	// Reuse this closure's scope unless a call is already using it. Evaluation
	// is single-threaded, so the only way to arrive here mid-call is a body
	// that reaches its own closure again; that case falls back to a fresh
	// scope rather than corrupting the outer one.
	var childEnv *env
	reused := !lc.inCall
	if reused {
		if lc.scratch == nil {
			lc.scratch = newEnv(lc.env)
			lc.scratch.reusable = true
		} else {
			lc.scratch.reset(lc.env)
		}
		childEnv = lc.scratch
		lc.inCall = true
		defer func() { lc.inCall = false }()
	} else {
		childEnv = newEnv(lc.env)
		childEnv.reusable = true // a nested closure must still snapshot it
	}

	for i, name := range lc.params {
		if i < len(args) {
			childEnv.set(name, args[i])
		}
	}
	ec := &evalCtx{ctx: ctx, env: childEnv, depth: lc.depth + 1, start: lc.start, debug: lc.debug}
	if err := ec.checkLimits(lc.executor); err != nil {
		return nil, err
	}
	return lc.executor.evalExpr(ec, lc.body)
}

func (ex *Executor) evalObject(ec *evalCtx, e *ast.ObjectExpr) (any, error) {
	obj := make(map[string]any, len(e.Fields))
	for _, field := range e.Fields {
		val, err := ex.evalExpr(ec, field.Value)
		if err != nil {
			return nil, err
		}
		obj[field.Key] = val
	}
	return obj, nil
}

func (ex *Executor) evalArray(ec *evalCtx, e *ast.ArrayExpr) (any, error) {
	arr := make([]any, 0, len(e.Elements))
	for _, elem := range e.Elements {
		val, err := ex.evalExpr(ec, elem)
		if err != nil {
			return nil, err
		}
		arr = append(arr, val)
	}
	return arr, nil
}

func (ex *Executor) evalIndex(ec *evalCtx, e *ast.IndexExpr) (any, error) {
	obj, err := ex.evalExpr(ec, e.Object)
	if err != nil {
		return nil, err
	}
	idx, err := ex.evalExpr(ec, e.Index)
	if err != nil {
		return nil, err
	}

	switch o := obj.(type) {
	case []any:
		i := toInt(idx)
		if i < 0 || i >= int64(len(o)) {
			return nil, nil // out of bounds = null
		}
		return o[i], nil
	case map[string]any:
		key := toString(idx)
		return o[key], nil
	default:
		return nil, fmt.Errorf("cannot index into %T", obj)
	}
}

func (ex *Executor) evalFieldAccess(ec *evalCtx, e *ast.FieldAccessExpr) (any, error) {
	obj, err := ex.evalExpr(ec, e.Object)
	if err != nil {
		return nil, err
	}

	if obj == nil {
		if e.Optional {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot access field %q on null", e.Field)
	}

	if m, ok := obj.(map[string]any); ok {
		return m[e.Field], nil
	}
	return nil, fmt.Errorf("cannot access field %q on %T", e.Field, obj)
}

func (ex *Executor) evalTry(ec *evalCtx, e *ast.TryExpr) (any, error) {
	val, err := ex.evalExpr(ec, e.Expr)
	if err != nil {
		if e.Default != nil {
			return ex.evalExpr(ec, e.Default)
		}
		return nil, nil
	}
	return val, nil
}

func (ex *Executor) evalCoalesce(ec *evalCtx, e *ast.CoalesceExpr) (any, error) {
	left, err := ex.evalExpr(ec, e.Left)
	if err != nil {
		return nil, err
	}
	if left != nil {
		return left, nil
	}
	return ex.evalExpr(ec, e.Right)
}

func (ex *Executor) evalInterpolatedString(ec *evalCtx, e *ast.InterpolatedStringExpr) (any, error) {
	var sb strings.Builder
	for _, part := range e.Parts {
		val, err := ex.evalExpr(ec, part)
		if err != nil {
			return nil, err
		}
		sb.WriteString(toString(val))
	}
	return sb.String(), nil
}

// --- For Expression ---

// ErrUserRaise is returned when a DTL function executes a raise statement.
var ErrUserRaise = errors.New("raise")

func (ex *Executor) evalFor(ec *evalCtx, e *ast.ForExpr) (any, error) {
	iterable, err := ex.evalExpr(ec, e.Iterable)
	if err != nil {
		return nil, err
	}

	arr, ok := iterable.([]any)
	if !ok {
		return nil, fmt.Errorf("for..in: expected array, got %T", iterable)
	}

	result := make([]any, 0, len(arr))
	for i, item := range arr {
		if err := ec.checkLimits(ex); err != nil {
			return nil, err
		}
		childEnv := newEnv(ec.env)
		childEnv.set(e.Variable, item)
		if e.Index != "" {
			childEnv.set(e.Index, int64(i))
		}
		childEc := &evalCtx{ctx: ec.ctx, env: childEnv, depth: ec.depth, start: ec.start, debug: ec.debug}
		val, err := ex.evalExpr(childEc, e.Body)
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}
	return result, nil
}

func (ex *Executor) evalRaise(ec *evalCtx, e *ast.RaiseExpr) (any, error) {
	msg, err := ex.evalExpr(ec, e.Message)
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%w: %s", ErrUserRaise, toString(msg))
}

func (ex *Executor) evalIn(ec *evalCtx, e *ast.InExpr) (any, error) {
	value, err := ex.evalExpr(ec, e.Value)
	if err != nil {
		return nil, err
	}
	collection, err := ex.evalExpr(ec, e.Collection)
	if err != nil {
		return nil, err
	}

	switch c := collection.(type) {
	case []any:
		for _, item := range c {
			if equalValues(value, item) {
				return true, nil
			}
		}
		return false, nil
	case map[string]any:
		key := toString(value)
		_, exists := c[key]
		return exists, nil
	default:
		return false, nil
	}
}

// --- Record Validation ---

// validateRecordArg checks that a value conforms to a record type definition.
// It validates field presence (required vs optional) and field types recursively.
func validateRecordArg(path string, value any, tn ast.TypeNode) error {
	if len(tn.Fields) == 0 {
		return nil // not a record type, skip validation
	}

	obj, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: expected a record, got %T", path, value)
	}

	for _, field := range tn.Fields {
		fieldPath := path + "." + field.Name
		val, exists := obj[field.Name]

		if !exists || val == nil {
			if !field.Optional {
				return fmt.Errorf("%s: missing required field %q", path, field.Name)
			}
			continue
		}

		if err := validateFieldType(fieldPath, val, field.Type); err != nil {
			return err
		}
	}
	return nil
}

// validateFieldType checks a single value against its declared type.
func validateFieldType(path string, val any, tn ast.TypeNode) error {
	// Handle arrays: validate each element
	if tn.IsArray {
		arr, ok := val.([]any)
		if !ok {
			return fmt.Errorf("%s: expected an array, got %T", path, val)
		}
		elemType := ast.TypeNode{Name: tn.Name, Fields: tn.Fields} // same type without IsArray
		for i, elem := range arr {
			elemPath := fmt.Sprintf("%s[%d]", path, i)
			if err := validateFieldType(elemPath, elem, elemType); err != nil {
				return err
			}
		}
		return nil
	}

	// Nested record
	if tn.Name == "record" && len(tn.Fields) > 0 {
		return validateRecordArg(path, val, tn)
	}

	// Primitive type checks
	switch tn.Name {
	case "string":
		if _, ok := val.(string); !ok {
			return fmt.Errorf("%s: expected string, got %T", path, val)
		}
	case "int":
		switch val.(type) {
		case int64, int, float64:
			// accept numeric types
		default:
			return fmt.Errorf("%s: expected int, got %T", path, val)
		}
	case "float", "number":
		switch val.(type) {
		case float64, int64, int:
			// accept numeric types
		default:
			return fmt.Errorf("%s: expected float, got %T", path, val)
		}
	case "bool":
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("%s: expected bool, got %T", path, val)
		}
	case "object":
		if _, ok := val.(map[string]any); !ok {
			return fmt.Errorf("%s: expected object, got %T", path, val)
		}
	case "any", "":
		// accept anything
	}
	return nil
}

// --- Type Coercion Helpers ---

// ToFloat converts a value to float64, used by stdlib functions.
func ToFloat(v any) float64 {
	return toFloat(v)
}

// ToInt converts a value to int64, used by stdlib functions.
func ToInt(v any) int64 {
	return toInt(v)
}

// ToBool converts a value to bool, used by stdlib functions.
func ToBool(v any) bool {
	return toBool(v)
}

// ToString converts a value to string, used by stdlib functions.
func ToString(v any) string {
	return toString(v)
}

// CallLambda invokes a lambda closure. Used by stdlib collection functions.
func CallLambda(ctx context.Context, val any, args []any, start time.Time, depth int) (any, error) {
	lc, ok := val.(*lambdaClosure)
	if !ok {
		return nil, fmt.Errorf("expected lambda, got %T", val)
	}
	return lc.Call(ctx, args, start, depth)
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	case bool:
		if n {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func toInt(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case bool:
		if n {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func toBool(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case nil:
		return false
	case int64:
		return b != 0
	case float64:
		return b != 0
	case string:
		return b != ""
	case []any:
		return len(b) > 0
	default:
		return true
	}
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case bool:
		if s {
			return "true"
		}
		return "false"
	case int64:
		return fmt.Sprintf("%d", s)
	case float64:
		if s == math.Trunc(s) && !math.IsInf(s, 0) {
			return fmt.Sprintf("%.1f", s)
		}
		return fmt.Sprintf("%g", s)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// --- Arithmetic ---

func evalArith(op string, left, right any) (any, error) {
	// Both int → int arithmetic
	li, liOk := asInt64(left)
	ri, riOk := asInt64(right)
	if liOk && riOk {
		switch op {
		case "+":
			return li + ri, nil
		case "-":
			return li - ri, nil
		case "*":
			return li * ri, nil
		case "/":
			if ri == 0 {
				return nil, ErrDivZero
			}
			return li / ri, nil
		case "%":
			if ri == 0 {
				return nil, ErrDivZero
			}
			return li % ri, nil
		}
	}

	// Fall through to float arithmetic
	lf := toFloat(left)
	rf := toFloat(right)
	switch op {
	case "+":
		return lf + rf, nil
	case "-":
		return lf - rf, nil
	case "*":
		return lf * rf, nil
	case "/":
		if rf == 0 {
			return nil, ErrDivZero
		}
		return lf / rf, nil
	case "%":
		if rf == 0 {
			return nil, ErrDivZero
		}
		return math.Mod(lf, rf), nil
	default:
		return nil, fmt.Errorf("unknown arithmetic operator: %s", op)
	}
}

func negateValue(v any) (any, error) {
	switch n := v.(type) {
	case int64:
		return -n, nil
	case float64:
		return -n, nil
	default:
		return -toFloat(v), nil
	}
}

func equalValues(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Numeric comparison: promote to float if mixed
	af, aNum := asNumeric(a)
	bf, bNum := asNumeric(b)
	if aNum && bNum {
		return af == bf
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func compareValues(op string, left, right any) (bool, error) {
	lf, lNum := asNumeric(left)
	rf, rNum := asNumeric(right)
	if lNum && rNum {
		switch op {
		case "<":
			return lf < rf, nil
		case ">":
			return lf > rf, nil
		case "<=":
			return lf <= rf, nil
		case ">=":
			return lf >= rf, nil
		case "==":
			return lf == rf, nil
		case "!=":
			return lf != rf, nil
		}
	}

	// String comparison
	ls, lStr := left.(string)
	rs, rStr := right.(string)
	if lStr && rStr {
		switch op {
		case "<":
			return ls < rs, nil
		case ">":
			return ls > rs, nil
		case "<=":
			return ls <= rs, nil
		case ">=":
			return ls >= rs, nil
		case "==":
			return ls == rs, nil
		case "!=":
			return ls != rs, nil
		}
	}

	return false, fmt.Errorf("cannot compare %T and %T with %s", left, right, op)
}

func asInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

func asNumeric(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}
