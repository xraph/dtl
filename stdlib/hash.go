package stdlib

import (
	"crypto/md5"  //#nosec G501 -- see the note on registerHash
	"crypto/sha1" //#nosec G505 -- see the note on registerHash
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash/crc32"

	"github.com/xraph/dtl/executor"
)

// Hashes, for interop rather than for security.
//
// A transformation that has to match a legacy checksum, reproduce a
// third-party API's request signature, or bucket records by a stable digest
// needs whatever algorithm the other side already chose, and that is often md5
// or sha1. Refusing to ship them would not make those systems go away; it would
// only push the work into a host builtin where it gets less scrutiny.
//
// So they are here, and their documentation says plainly what they are not
// suitable for. md5 and sha1 are broken for any purpose that depends on an
// adversary being unable to find a collision — signatures, deduplicating
// attacker-supplied content, anything standing in for authentication. crc32 is
// a checksum and was never a cryptographic function at all.
func registerHash(m map[string]*executor.BuiltinFunc) {
	register(m, "hash::sha256", 1, 1, fnSHA256,
		"hash::sha256(s) -> string -- SHA-256 digest as lowercase hex")
	register(m, "hash::sha512", 1, 1, fnSHA512,
		"hash::sha512(s) -> string -- SHA-512 digest as lowercase hex")
	register(m, "hash::sha1", 1, 1, fnSHA1,
		"hash::sha1(s) -> string -- SHA-1 digest as lowercase hex. Broken against collisions: use only for interop, never for security")
	register(m, "hash::md5", 1, 1, fnMD5,
		"hash::md5(s) -> string -- MD5 digest as lowercase hex. Broken against collisions: use only for interop, never for security")
	register(m, "hash::crc32", 1, 1, fnCRC32,
		"hash::crc32(s) -> string -- CRC-32 checksum as lowercase hex. An error-detection checksum, not a secure hash")
}

func fnSHA256(args []any) (any, error) {
	sum := sha256.Sum256([]byte(executor.ToString(args[0])))
	return hex.EncodeToString(sum[:]), nil
}

func fnSHA512(args []any) (any, error) {
	sum := sha512.Sum512([]byte(executor.ToString(args[0])))
	return hex.EncodeToString(sum[:]), nil
}

// fnSHA1 is offered for interop with systems that already use SHA-1. It must
// not be used where collision resistance matters.
func fnSHA1(args []any) (any, error) {
	sum := sha1.Sum([]byte(executor.ToString(args[0]))) //#nosec G401 -- interop with existing SHA-1 consumers; documented as unsuitable for security
	return hex.EncodeToString(sum[:]), nil
}

// fnMD5 is offered for interop with systems that already use MD5. It must not
// be used where collision resistance matters.
func fnMD5(args []any) (any, error) {
	sum := md5.Sum([]byte(executor.ToString(args[0]))) //#nosec G401 -- interop with existing MD5 consumers; documented as unsuitable for security
	return hex.EncodeToString(sum[:]), nil
}

func fnCRC32(args []any) (any, error) {
	sum := crc32.ChecksumIEEE([]byte(executor.ToString(args[0])))
	return hex.EncodeToString([]byte{
		byte(sum >> 24), byte(sum >> 16), byte(sum >> 8), byte(sum),
	}), nil
}
