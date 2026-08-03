package stdlib

import (
	"strings"
	"testing"

	"github.com/xraph/dtl/executor"
)

func TestEncodings(t *testing.T) {
	tests := []struct {
		name string
		fn   func([]any) (any, error)
		in   string
		want string
	}{
		{"base64", fnBase64Encode, "hello", "aGVsbG8="},
		{"base64 empty", fnBase64Encode, "", ""},
		{"base64 multibyte", fnBase64Encode, "café", "Y2Fmw6k="},
		{"hex", fnHexEncode, "hello", "68656c6c6f"},
		{"hex empty", fnHexEncode, "", ""},
		{"url encode", fnURLEncode, "a b&c", "a+b%26c"},
		{"url encode slash", fnURLEncode, "a/b", "a%2Fb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fn([]any{tt.in})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEncodingsRoundTrip(t *testing.T) {
	pairs := []struct {
		name           string
		encode, decode func([]any) (any, error)
	}{
		{"base64", fnBase64Encode, fnBase64Decode},
		{"base64url", fnBase64URLEncode, fnBase64URLDecode},
		{"hex", fnHexEncode, fnHexDecode},
		{"url", fnURLEncode, fnURLDecode},
	}

	inputs := []string{"", "hello", "café", "a b&c/d+e", "🎉", strings.Repeat("x", 500)}

	for _, p := range pairs {
		for _, in := range inputs {
			t.Run(p.name+"/"+in[:min(len(in), 12)], func(t *testing.T) {
				encoded, err := p.encode([]any{in})
				if err != nil {
					t.Fatalf("encode: %v", err)
				}
				decoded, err := p.decode([]any{encoded})
				if err != nil {
					t.Fatalf("decode(%q): %v", encoded, err)
				}
				if decoded != in {
					t.Errorf("round trip changed %q into %q", in, decoded)
				}
			})
		}
	}
}

// Decoders report malformed input rather than returning empty, so corrupt data
// stops the transformation where the problem is instead of flowing onward
// looking like a legitimately empty value.
func TestDecodersRejectMalformedInput(t *testing.T) {
	tests := []struct {
		name string
		fn   func([]any) (any, error)
		in   string
	}{
		{"base64", fnBase64Decode, "not!valid!base64"},
		{"base64url", fnBase64URLDecode, "not!valid"},
		{"hex odd length", fnHexDecode, "abc"},
		{"hex non-hex", fnHexDecode, "zzzz"},
		{"url bad escape", fnURLDecode, "%zz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.fn([]any{tt.in})
			if err == nil {
				t.Fatal("expected an error for malformed input")
			}
			if !strings.Contains(err.Error(), "encoding::") {
				t.Errorf("error %q should name the function", err)
			}
		})
	}
}

// Digests are pinned against published vectors, so a refactor that changed the
// algorithm would be caught rather than silently producing different output.
func TestHashesMatchKnownVectors(t *testing.T) {
	tests := []struct {
		name string
		fn   func([]any) (any, error)
		in   string
		want string
	}{
		{"sha256 empty", fnSHA256, "",
			"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"sha256 abc", fnSHA256, "abc",
			"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{"sha512 abc", fnSHA512, "abc",
			"ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a" +
				"2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f"},
		{"sha1 abc", fnSHA1, "abc", "a9993e364706816aba3e25717850c26c9cd0d89d"},
		{"md5 abc", fnMD5, "abc", "900150983cd24fb0d6963f7d28e17f72"},
		{"md5 empty", fnMD5, "", "d41d8cd98f00b204e9800998ecf8427e"},
		{"crc32 abc", fnCRC32, "abc", "352441c2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fn([]any{tt.in})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// Every digest is lowercase hex of the expected width, so callers can compare
// them against digests from other systems without normalising first.
func TestHashesAreLowercaseHexOfExpectedWidth(t *testing.T) {
	tests := []struct {
		name  string
		fn    func([]any) (any, error)
		chars int
	}{
		{"sha256", fnSHA256, 64},
		{"sha512", fnSHA512, 128},
		{"sha1", fnSHA1, 40},
		{"md5", fnMD5, 32},
		{"crc32", fnCRC32, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fn([]any{"some input"})
			if err != nil {
				t.Fatal(err)
			}
			s, _ := got.(string)
			if len(s) != tt.chars {
				t.Errorf("got %d characters, want %d (%q)", len(s), tt.chars, s)
			}
			if s != strings.ToLower(s) {
				t.Errorf("digest %q is not lowercase", s)
			}
			if strings.Trim(s, "0123456789abcdef") != "" {
				t.Errorf("digest %q contains non-hex characters", s)
			}
		})
	}
}

// The weak algorithms ship for interop, so their documentation has to say so
// plainly. This asserts the warning is actually there, since it is the only
// thing standing between the function and a caller reaching for it by habit.
func TestWeakHashesAreDocumentedAsUnsuitableForSecurity(t *testing.T) {
	builtins := make(map[string]*executor.BuiltinFunc)
	RegisterAll(builtins)

	for _, name := range []string{"hash::md5", "hash::sha1", "hash::crc32"} {
		t.Run(name, func(t *testing.T) {
			b, ok := builtins[name]
			if !ok {
				t.Fatalf("%s is not registered", name)
			}
			doc := strings.ToLower(b.Doc)
			if !strings.Contains(doc, "never for security") &&
				!strings.Contains(doc, "not a secure hash") {
				t.Errorf("%s doc %q must warn against security use", name, b.Doc)
			}
		})
	}
}

// The URL-safe alphabet exists precisely to avoid + and /, which have meaning
// inside a URL. This is the difference between the two base64 pairs.
func TestBase64URLAvoidsURLUnsafeCharacters(t *testing.T) {
	// Bytes chosen to land on the alphabet positions that differ: standard
	// base64 encodes them with + and /, URL-safe with - and _.
	input := string([]byte{0xfb, 0xef, 0xbe})

	std, err := fnBase64Encode([]any{input})
	if err != nil {
		t.Fatal(err)
	}
	safe, err := fnBase64URLEncode([]any{input})
	if err != nil {
		t.Fatal(err)
	}

	stdStr, _ := std.(string)
	safeStr, _ := safe.(string)

	if !strings.ContainsAny(stdStr, "+/") {
		t.Fatalf("test input %q does not exercise the differing characters (standard form %q)", input, stdStr)
	}
	if strings.ContainsAny(safeStr, "+/") {
		t.Errorf("URL-safe form %q still contains + or /", safeStr)
	}
	if !strings.ContainsAny(safeStr, "-_") {
		t.Errorf("URL-safe form %q should use - or _", safeStr)
	}
}
