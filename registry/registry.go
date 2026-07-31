package registry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xraph/dtl/ast"
	"github.com/xraph/dtl/compiler"
	"github.com/xraph/dtl/executor"
	"github.com/xraph/dtl/parser"

	"github.com/xraph/dtl/stdlib"
)

// Config holds registry configuration.
type Config struct {
	DefaultTimeout time.Duration
	MaxCallDepth   int
}

// ExecuteResult holds the execution result along with any debug output.
type ExecuteResult struct {
	Value any                   `json:"result"`
	Logs  []executor.DebugEntry `json:"logs,omitempty"`
}

// GetValue returns the execution result value.
// Implements the DTLExecuteResult interface used by the formula extension.
func (r *ExecuteResult) GetValue() any {
	if r == nil {
		return nil
	}
	return r.Value
}

// FunctionLoader loads a function's source from external storage (e.g. database)
// by its fully-qualified name. The context carries workspace scope.
type FunctionLoader func(ctx context.Context, fullName string) (string, error)

// Registry is the in-memory compiled function cache and execution engine.
// It owns the compiler, executor, and built-in function table.
type Registry struct {
	builtins map[string]*executor.BuiltinFunc
	compiled map[string]*executor.CompiledFunction
	mu       sync.RWMutex
	executor *executor.Executor
	config   Config
	loader   FunctionLoader // optional: loads functions from DB on cache miss
}

// New creates a registry with stdlib loaded and ready for function registration.
func New(config Config) *Registry {
	builtins := make(map[string]*executor.BuiltinFunc)
	stdlib.RegisterAll(builtins)

	r := &Registry{
		builtins: builtins,
		compiled: make(map[string]*executor.CompiledFunction),
		config:   config,
	}

	// Create the executor with the registry as function lookup
	r.executor = executor.New(builtins, r, executor.ExecConfig{
		Timeout:  config.DefaultTimeout,
		MaxDepth: config.MaxCallDepth,
	})

	return r
}

// --- compiler.FunctionResolver implementation ---

// ResolveFunction checks if a function name is known (built-in or user-defined).
func (r *Registry) ResolveFunction(name string) (int, bool) {
	// Check builtins
	if bf, ok := r.builtins[name]; ok {
		return bf.MinArgs, true
	}
	// Check compiled user functions
	r.mu.RLock()
	defer r.mu.RUnlock()
	if cf, ok := r.compiled[name]; ok {
		return len(cf.AST.Params), true
	}
	return 0, false
}

// ListFunctionNames returns all known function names (built-in and user-defined).
// Implements compiler.FunctionLister for "did you mean?" suggestions.
func (r *Registry) ListFunctionNames() []string {
	names := make([]string, 0, len(r.builtins))
	for name := range r.builtins {
		names = append(names, name)
	}
	r.mu.RLock()
	for name := range r.compiled {
		names = append(names, name)
	}
	r.mu.RUnlock()
	return names
}

// SetLoader sets an optional function loader that is called when GetCompiled
// misses the in-memory cache. The loader typically queries the database.
func (r *Registry) SetLoader(loader FunctionLoader) {
	r.loader = loader
}

// --- executor.FunctionLookup implementation ---

// GetCompiled returns a compiled user-defined function by full name.
// If not found in the in-memory cache, it falls back to the loader (if set)
// to fetch and register the function from the database.
func (r *Registry) GetCompiled(ctx context.Context, name string) (*executor.CompiledFunction, bool) {
	r.mu.RLock()
	cf, ok := r.compiled[name]
	r.mu.RUnlock()
	if ok {
		return cf, true
	}

	// Fall back to loader (e.g. database) if available
	if r.loader != nil {
		source, err := r.loader(ctx, name)
		if err != nil {
			return nil, false
		}
		// Register it so future lookups hit the cache
		if regErr := r.Register(name, source); regErr != nil {
			return nil, false
		}
		r.mu.RLock()
		cf, ok = r.compiled[name]
		r.mu.RUnlock()
		return cf, ok
	}

	return nil, false
}

// --- Registration ---

// Register parses, compiles, and caches a function from its source.
// Uses a loader-backed resolver when available so that cross-namespace
// dependencies are correctly tracked even for functions not yet in cache.
func (r *Registry) Register(fullName, source string) error {
	fnAST, parseErrs := parser.Parse(source)
	if len(parseErrs) > 0 {
		return fmt.Errorf("parse error: %s", parseErrs[0].Error())
	}

	// Use a resolver that falls back to the loader (DB) so dependencies
	// on functions not yet in the in-memory cache are still discovered.
	var resolver compiler.FunctionResolver = r
	if r.loader != nil {
		resolver = &registerResolver{registry: r}
	}

	comp := compiler.New(resolver)
	result := comp.Compile(fnAST)
	if result.HasErrors() {
		return fmt.Errorf("compile error: %s", result.Errors[0].Error())
	}

	cf := &executor.CompiledFunction{
		Name:   fullName,
		AST:    fnAST,
		Deps:   result.Dependencies,
		Source: source,
	}

	r.mu.Lock()
	r.compiled[fullName] = cf
	r.mu.Unlock()

	return nil
}

// registerResolver wraps the registry and falls back to the loader
// during registration/compilation to resolve functions in the database.
type registerResolver struct {
	registry *Registry
}

func (rr *registerResolver) ResolveFunction(name string) (int, bool) {
	// Check in-memory first (builtins + compiled)
	if arity, ok := rr.registry.ResolveFunction(name); ok {
		return arity, true
	}
	// Fall back to loader — attempt to verify the function exists in DB.
	// Use a background context since registration doesn't have request scope.
	// This is best-effort: if the loader needs workspace context it may fail,
	// in which case we simply report the function as unknown (same as before).
	if rr.registry.loader != nil {
		source, err := rr.registry.loader(context.Background(), name)
		if err == nil && source != "" {
			// Function exists in DB — parse to count params
			fnAST, parseErrs := parser.Parse(source)
			if len(parseErrs) == 0 && fnAST != nil {
				return len(fnAST.Params), true
			}
			// Parseable but errors — still report as existing
			return 0, true
		}
	}
	return 0, false
}

// RegisterBuiltin adds a Go-implemented function to the built-in table.
func (r *Registry) RegisterBuiltin(name string, bf *executor.BuiltinFunc) {
	r.builtins[name] = bf
}

// Invalidate removes a compiled function from the cache.
func (r *Registry) Invalidate(fullName string) {
	r.mu.Lock()
	delete(r.compiled, fullName)
	r.mu.Unlock()
}

// --- Execution ---

// Execute runs a named function with the given argument map.
func (r *Registry) Execute(ctx context.Context, fullName string, args map[string]any) (*ExecuteResult, error) {
	cf, ok := r.GetCompiled(ctx, fullName)
	if !ok {
		return nil, fmt.Errorf("function %q not found in registry", fullName)
	}

	args = r.injectGlobalVars(ctx, args)
	debugCtx, debugBuf := executor.NewDebugContext(ctx)
	result, err := r.executor.Execute(debugCtx, cf, args)
	if err != nil {
		return &ExecuteResult{Logs: *debugBuf}, err
	}

	return &ExecuteResult{Value: result, Logs: *debugBuf}, nil
}

// ExecuteInline parses, compiles, and executes a DTL source string without caching.
func (r *Registry) ExecuteInline(ctx context.Context, source string, args map[string]any) (*ExecuteResult, error) {
	fnAST, parseErrs := parser.Parse(source)
	if len(parseErrs) > 0 {
		return nil, fmt.Errorf("parse error: %s", parseErrs[0].Error())
	}

	comp := compiler.New(r)
	compResult := comp.Compile(fnAST)
	if compResult.HasErrors() {
		return nil, fmt.Errorf("compile error: %s", compResult.Errors[0].Error())
	}

	cf := &executor.CompiledFunction{
		Name:   "_inline",
		AST:    fnAST,
		Deps:   compResult.Dependencies,
		Source: source,
	}

	args = r.injectGlobalVars(ctx, args)
	debugCtx, debugBuf := executor.NewDebugContext(ctx)
	result, err := r.executor.Execute(debugCtx, cf, args)
	if err != nil {
		return &ExecuteResult{Logs: *debugBuf}, err
	}

	return &ExecuteResult{Value: result, Logs: *debugBuf}, nil
}

// injectGlobalVars adds the "global" object to args if present in context.
func (r *Registry) injectGlobalVars(ctx context.Context, args map[string]any) map[string]any {
	gv := stdlib.GlobalVarsFrom(ctx)
	if gv == nil {
		return args
	}
	if args == nil {
		args = make(map[string]any)
	}
	// Don't overwrite if caller explicitly provided "global"
	if _, exists := args["global"]; !exists {
		args["global"] = gv
	}
	return args
}

// --- Validation ---

// ValidateResult contains the result of source validation.
type ValidateResult struct {
	Valid  bool                    `json:"valid"`
	Errors []compiler.CompileError `json:"errors,omitempty"`
	AST    *ast.FnAST              `json:"-"`
}

// dbBackedResolver wraps the registry and falls back to a lookup function
// for functions not yet compiled in the in-memory cache.
type dbBackedResolver struct {
	registry   *Registry
	lookupFunc func(fullName string) (int, bool)
}

func (d *dbBackedResolver) ResolveFunction(name string) (int, bool) {
	if arity, ok := d.registry.ResolveFunction(name); ok {
		return arity, true
	}
	if d.lookupFunc != nil {
		return d.lookupFunc(name)
	}
	return 0, false
}

// Validate parses and compiles source, returning validation errors.
func (r *Registry) Validate(source string) *ValidateResult {
	return r.ValidateWithLookup(source, nil)
}

// ValidateWithLookup parses and compiles source, using an optional lookup function
// to resolve functions not found in the in-memory registry (e.g. from the database).
func (r *Registry) ValidateWithLookup(source string, lookup func(string) (int, bool)) *ValidateResult {
	fnAST, parseErrs := parser.Parse(source)

	result := &ValidateResult{Valid: true}

	// Convert parse errors to compile errors
	for _, pe := range parseErrs {
		result.Errors = append(result.Errors, compiler.CompileError{
			Line:    pe.Pos.Line,
			Column:  pe.Pos.Column,
			Code:    "parse_error",
			Message: pe.Message,
		})
	}

	if fnAST == nil {
		result.Valid = false
		return result
	}

	// Build resolver — use DB-backed resolver if lookup provided, otherwise registry only
	var resolver compiler.FunctionResolver
	if lookup != nil {
		resolver = &dbBackedResolver{registry: r, lookupFunc: lookup}
	} else {
		resolver = r
	}

	// Run compiler validation
	comp := compiler.New(resolver)
	compResult := comp.Compile(fnAST)
	result.Errors = append(result.Errors, compResult.Errors...)
	result.AST = fnAST
	result.Valid = len(result.Errors) == 0

	return result
}

// --- Accessors ---

// GetBuiltins returns the built-in function table (read-only use).
func (r *Registry) GetBuiltins() map[string]*executor.BuiltinFunc {
	return r.builtins
}

// ListCompiled returns all currently compiled function names.
func (r *Registry) ListCompiled() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.compiled))
	for name := range r.compiled {
		names = append(names, name)
	}
	return names
}
