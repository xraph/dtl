package stdlib

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"

	"github.com/xraph/dtl/executor"
)

// Encodings a transformation runs into at a system boundary: a base64 payload
// from an API, a hex digest, a query parameter that has to survive a URL.
func registerEncoding(m map[string]*executor.BuiltinFunc) {
	register(m, "encoding::base64_encode", 1, 1, fnBase64Encode,
		"encoding::base64_encode(s) -> string -- Standard base64 with padding")
	register(m, "encoding::base64_decode", 1, 1, fnBase64Decode,
		"encoding::base64_decode(s) -> string -- Decodes standard base64. Errors on malformed input")
	register(m, "encoding::base64url_encode", 1, 1, fnBase64URLEncode,
		"encoding::base64url_encode(s) -> string -- URL-safe base64, using - and _ instead of + and /")
	register(m, "encoding::base64url_decode", 1, 1, fnBase64URLDecode,
		"encoding::base64url_decode(s) -> string -- Decodes URL-safe base64. Errors on malformed input")
	register(m, "encoding::hex_encode", 1, 1, fnHexEncode,
		"encoding::hex_encode(s) -> string -- Lowercase hexadecimal")
	register(m, "encoding::hex_decode", 1, 1, fnHexDecode,
		"encoding::hex_decode(s) -> string -- Decodes hexadecimal. Errors on malformed input")
	register(m, "encoding::url_encode", 1, 1, fnURLEncode,
		"encoding::url_encode(s) -> string -- Percent-encodes a string for use in a URL query")
	register(m, "encoding::url_decode", 1, 1, fnURLDecode,
		"encoding::url_decode(s) -> string -- Decodes percent-encoding. Errors on malformed input")
}

// Decoders report their errors rather than returning empty. A silently empty
// result would let corrupt input flow onward looking like a legitimately empty
// value; an error stops the transformation where the problem is, and `try` is
// there for callers who would rather carry on.

func fnBase64Encode(args []any) (any, error) {
	return base64.StdEncoding.EncodeToString([]byte(executor.ToString(args[0]))), nil
}

func fnBase64Decode(args []any) (any, error) {
	decoded, err := base64.StdEncoding.DecodeString(executor.ToString(args[0]))
	if err != nil {
		return nil, fmt.Errorf("encoding::base64_decode: %w", err)
	}
	return string(decoded), nil
}

func fnBase64URLEncode(args []any) (any, error) {
	return base64.URLEncoding.EncodeToString([]byte(executor.ToString(args[0]))), nil
}

func fnBase64URLDecode(args []any) (any, error) {
	decoded, err := base64.URLEncoding.DecodeString(executor.ToString(args[0]))
	if err != nil {
		return nil, fmt.Errorf("encoding::base64url_decode: %w", err)
	}
	return string(decoded), nil
}

func fnHexEncode(args []any) (any, error) {
	return hex.EncodeToString([]byte(executor.ToString(args[0]))), nil
}

func fnHexDecode(args []any) (any, error) {
	decoded, err := hex.DecodeString(executor.ToString(args[0]))
	if err != nil {
		return nil, fmt.Errorf("encoding::hex_decode: %w", err)
	}
	return string(decoded), nil
}

// fnURLEncode uses query escaping, which encodes a space as '+'. That is the
// right form for the query string, which is where a transformation building a
// URL almost always needs it.
func fnURLEncode(args []any) (any, error) {
	return url.QueryEscape(executor.ToString(args[0])), nil
}

func fnURLDecode(args []any) (any, error) {
	decoded, err := url.QueryUnescape(executor.ToString(args[0]))
	if err != nil {
		return nil, fmt.Errorf("encoding::url_decode: %w", err)
	}
	return decoded, nil
}
