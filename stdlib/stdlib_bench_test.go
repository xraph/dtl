package stdlib

import (
	"testing"

	"github.com/xraph/dtl/executor"
)

func benchCall(b *testing.B, name string, args ...any) {
	b.Helper()
	m := make(map[string]*executor.BuiltinFunc)
	RegisterAll(m)
	fn := m[name]
	if fn == nil {
		b.Fatalf("builtin %q not found", name)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := fn.Fn(args)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuiltin_Len(b *testing.B) {
	benchCall(b, "len", []any{1, 2, 3, 4, 5})
}

func BenchmarkBuiltin_Upper(b *testing.B) {
	benchCall(b, "upper", "hello world this is a test")
}

func BenchmarkBuiltin_Replace(b *testing.B) {
	benchCall(b, "replace", "hello world hello world", "hello", "goodbye")
}

func BenchmarkBuiltin_Sum(b *testing.B) {
	arr := make([]any, 1000)
	for i := range arr {
		arr[i] = float64(i)
	}
	benchCall(b, "sum", arr)
}

func BenchmarkBuiltin_Sort(b *testing.B) {
	arr := make([]any, 100)
	for i := range arr {
		arr[i] = float64(100 - i)
	}
	benchCall(b, "sort", arr)
}

func BenchmarkBuiltin_FormatNumber(b *testing.B) {
	benchCall(b, "format_number", 1234567.89, int64(2), ",")
}

func BenchmarkBuiltin_FormatCurrency(b *testing.B) {
	benchCall(b, "format_currency", 1234.50, "USD")
}

func BenchmarkBuiltin_Keys(b *testing.B) {
	obj := map[string]any{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5}
	benchCall(b, "keys", obj)
}

func BenchmarkBuiltin_Includes(b *testing.B) {
	arr := make([]any, 100)
	for i := range arr {
		arr[i] = int64(i)
	}
	benchCall(b, "includes", arr, int64(99))
}
