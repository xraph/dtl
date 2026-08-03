package benchmarks

import (
	"context"
	"fmt"

	"github.com/dop251/goja"
	"github.com/expr-lang/expr"
	exprvm "github.com/expr-lang/expr/vm"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
	lua "github.com/yuin/gopher-lua"
	"go.starlark.net/starlark"

	"github.com/xraph/dtl/registry"
)

// Program is a compiled workload, ready to be evaluated repeatedly.
// Every engine here separates compilation from evaluation, which is how a host
// embeds them: compile once at load, evaluate per request. Benchmarking
// compile+eval together would measure something nobody runs in a hot path.
type Program interface {
	Eval(in map[string]any) (any, error)
}

// Engine compiles one workload into a reusable Program.
type Engine interface {
	Name() string
	Source(w *Workload) string
	Compile(w *Workload) (Program, error)
}

// Engines is the comparison set: DTL plus five widely embedded Go languages.
var Engines = []Engine{
	dtlEngine{}, exprEngine{}, celEngine{}, gojaEngine{}, luaEngine{}, starlarkEngine{},
}

// --- DTL ---

type dtlEngine struct{}

func (dtlEngine) Name() string              { return "DTL" }
func (dtlEngine) Source(w *Workload) string { return w.DTL }

func (dtlEngine) Compile(w *Workload) (Program, error) {
	reg := registry.New(registry.Config{})
	name := "bench::" + w.Name
	if err := reg.Register(name, w.DTL); err != nil {
		return nil, err
	}
	return &dtlProgram{reg: reg, name: name}, nil
}

type dtlProgram struct {
	reg  *registry.Registry
	name string
}

func (p *dtlProgram) Eval(in map[string]any) (any, error) {
	res, err := p.reg.Execute(context.Background(), p.name, in)
	if err != nil {
		return nil, err
	}
	return res.Value, nil
}

// --- expr-lang/expr (bytecode VM) ---

type exprEngine struct{}

func (exprEngine) Name() string              { return "expr" }
func (exprEngine) Source(w *Workload) string { return w.Expr }

func (exprEngine) Compile(w *Workload) (Program, error) {
	p, err := expr.Compile(w.Expr, expr.Env(w.Input))
	if err != nil {
		return nil, err
	}
	return &exprProgram{p: p}, nil
}

type exprProgram struct{ p *exprvm.Program }

func (p *exprProgram) Eval(in map[string]any) (any, error) { return expr.Run(p.p, in) }

// --- google/cel-go ---

type celEngine struct{}

func (celEngine) Name() string              { return "cel-go" }
func (celEngine) Source(w *Workload) string { return w.CEL }

func celTypeOf(v any) *cel.Type {
	switch v.(type) {
	case float64:
		return cel.DoubleType
	case string:
		return cel.StringType
	case bool:
		return cel.BoolType
	case map[string]any:
		return cel.MapType(cel.StringType, cel.DynType)
	case []any:
		return cel.ListType(cel.DoubleType)
	}
	return cel.DynType
}

func (celEngine) Compile(w *Workload) (Program, error) {
	opts := []cel.EnvOption{ext.Strings()}
	for name, v := range w.Input {
		opts = append(opts, cel.Variable(name, celTypeOf(v)))
	}
	env, err := cel.NewEnv(opts...)
	if err != nil {
		return nil, err
	}
	ast, iss := env.Compile(w.CEL)
	if iss != nil && iss.Err() != nil {
		return nil, iss.Err()
	}
	prg, err := env.Program(ast)
	if err != nil {
		return nil, err
	}
	return &celProgram{prg: prg}, nil
}

type celProgram struct{ prg cel.Program }

func (p *celProgram) Eval(in map[string]any) (any, error) {
	out, _, err := p.prg.Eval(in)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// --- dop251/goja (JavaScript) ---

type gojaEngine struct{}

func (gojaEngine) Name() string              { return "goja(JS)" }
func (gojaEngine) Source(w *Workload) string { return w.JS }

func (gojaEngine) Compile(w *Workload) (Program, error) {
	// Wrap in a function so evaluation is a call with arguments rather than a
	// fresh global-scope program run — the same shape as every other engine.
	params := make([]string, 0, len(w.Input))
	for name := range w.Input {
		params = append(params, name)
	}
	src := fmt.Sprintf("(function (%s) { return %s; })", joinComma(params), w.JS)
	prog, err := goja.Compile(w.Name, src, true)
	if err != nil {
		return nil, err
	}
	vm := goja.New()
	v, err := vm.RunProgram(prog)
	if err != nil {
		return nil, err
	}
	fn, ok := goja.AssertFunction(v)
	if !ok {
		return nil, fmt.Errorf("goja: compiled value is not a function")
	}
	return &gojaProgram{vm: vm, fn: fn, params: params}, nil
}

type gojaProgram struct {
	vm     *goja.Runtime
	fn     goja.Callable
	params []string
}

func (p *gojaProgram) Eval(in map[string]any) (any, error) {
	args := make([]goja.Value, len(p.params))
	for i, name := range p.params {
		args[i] = p.vm.ToValue(in[name])
	}
	v, err := p.fn(goja.Undefined(), args...)
	if err != nil {
		return nil, err
	}
	return v.Export(), nil
}

// --- yuin/gopher-lua ---

type luaEngine struct{}

func (luaEngine) Name() string              { return "gopher-lua" }
func (luaEngine) Source(w *Workload) string { return w.Lua }

func (luaEngine) Compile(w *Workload) (Program, error) {
	L := lua.NewState()
	params := make([]string, 0, len(w.Input))
	for name := range w.Input {
		params = append(params, name)
	}
	src := fmt.Sprintf("function __f(%s)\n%s\nend", joinComma(params), w.Lua)
	if err := L.DoString(src); err != nil {
		L.Close()
		return nil, err
	}
	fn := L.GetGlobal("__f")
	if fn == lua.LNil {
		L.Close()
		return nil, fmt.Errorf("gopher-lua: __f not defined")
	}
	return &luaProgram{L: L, fn: fn, params: params}, nil
}

type luaProgram struct {
	L      *lua.LState
	fn     lua.LValue
	params []string
}

func toLua(L *lua.LState, v any) lua.LValue {
	switch t := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(t)
	case float64:
		return lua.LNumber(t)
	case string:
		return lua.LString(t)
	case []any:
		tbl := L.NewTable()
		for _, e := range t {
			tbl.Append(toLua(L, e))
		}
		return tbl
	case map[string]any:
		tbl := L.NewTable()
		for k, val := range t {
			tbl.RawSetString(k, toLua(L, val))
		}
		return tbl
	}
	return lua.LNil
}

func (p *luaProgram) Eval(in map[string]any) (any, error) {
	p.L.Push(p.fn)
	for _, name := range p.params {
		p.L.Push(toLua(p.L, in[name]))
	}
	if err := p.L.PCall(len(p.params), 1, nil); err != nil {
		return nil, err
	}
	ret := p.L.Get(-1)
	p.L.Pop(1)
	return ret, nil
}

// --- google/starlark-go ---

type starlarkEngine struct{}

func (starlarkEngine) Name() string              { return "starlark" }
func (starlarkEngine) Source(w *Workload) string { return w.Starlark }

func (starlarkEngine) Compile(w *Workload) (Program, error) {
	thread := &starlark.Thread{Name: w.Name}
	globals, err := starlark.ExecFile(thread, w.Name+".star", w.Starlark, nil)
	if err != nil {
		return nil, err
	}
	fn, ok := globals["f"].(starlark.Callable)
	if !ok {
		return nil, fmt.Errorf("starlark: f is not callable")
	}
	// Starlark has no **kwargs-by-map call helper that preserves order, so bind
	// positionally against a stable parameter order taken from the function.
	params := make([]string, 0, len(w.Input))
	for name := range w.Input {
		params = append(params, name)
	}
	return &starlarkProgram{fn: fn, params: params}, nil
}

type starlarkProgram struct {
	fn     starlark.Callable
	params []string
}

func toStarlark(v any) (starlark.Value, error) {
	switch t := v.(type) {
	case nil:
		return starlark.None, nil
	case bool:
		return starlark.Bool(t), nil
	case float64:
		return starlark.Float(t), nil
	case string:
		return starlark.String(t), nil
	case []any:
		els := make([]starlark.Value, len(t))
		for i, e := range t {
			sv, err := toStarlark(e)
			if err != nil {
				return nil, err
			}
			els[i] = sv
		}
		return starlark.NewList(els), nil
	case map[string]any:
		d := starlark.NewDict(len(t))
		for k, val := range t {
			sv, err := toStarlark(val)
			if err != nil {
				return nil, err
			}
			if err := d.SetKey(starlark.String(k), sv); err != nil {
				return nil, err
			}
		}
		return d, nil
	}
	return nil, fmt.Errorf("starlark: cannot convert %T", v)
}

func (p *starlarkProgram) Eval(in map[string]any) (any, error) {
	// Pass by keyword so the positional order of p.params cannot desync from
	// the Starlark function's own signature.
	kwargs := make([]starlark.Tuple, 0, len(p.params))
	for _, name := range p.params {
		sv, err := toStarlark(in[name])
		if err != nil {
			return nil, err
		}
		kwargs = append(kwargs, starlark.Tuple{starlark.String(name), sv})
	}
	thread := &starlark.Thread{}
	return starlark.Call(thread, p.fn, nil, kwargs)
}

// --- helpers ---

func joinComma(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}

// normalize collapses each engine's native result type to a plain Go value so
// results can be compared across engines. All numerics become float64.
func normalize(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case bool:
		return t
	case string:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case float32:
		return float64(t)
	case float64:
		return t
	case lua.LBool:
		return bool(t)
	case lua.LNumber:
		return float64(t)
	case lua.LString:
		return string(t)
	case starlark.Bool:
		return bool(t)
	case starlark.Float:
		return float64(t)
	case starlark.String:
		return string(t)
	case starlark.Int:
		i, _ := t.Int64()
		return float64(i)
	case ref.Val:
		return normalize(t.Value())
	case goja.Value:
		return normalize(t.Export())
	}
	return v
}
