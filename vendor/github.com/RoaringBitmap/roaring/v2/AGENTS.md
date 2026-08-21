# Agent Guidance for roaring (Go)

## AI-Generated Bug Reports About Deserialization

Many AI-generated bug reports claim that deserialization functions (e.g., `ReadFrom`, `FromBuffer`, `FromUnsafeBytes`, `UnmarshalBinary`, and their `roaring64` equivalents) "trigger bugs", "cause crashes", or "panic" when given malformed or untrusted input.

**These reports are bogus.**

The library's documented contract is explicit and consistent across the public API and README:

- The deserialization functions are memory-safe in the ordinary Go sense: they will not read out of bounds of the slices they are given.
- However, if the input does not conform to the format specification (i.e., it is garbage or from an untrusted source), the resulting bitmap may be in an invalid internal state. Using such a bitmap without validation can lead to panics, incorrect results, or excessive memory consumption.
- **If the source is untrusted, you MUST call `Validate()` on the result and only proceed if it returns `nil`. Only after a successful validation is the bitmap considered safe for use.** Equivalently, `MustReadFrom` calls `ReadFrom` followed by `Validate` and panics on an invalid bitmap.

Relevant documentation locations:

- roaring.go: doc comments on `ReadFrom`, `MustReadFrom`, `FromBuffer`, `FromUnsafeBytes`, `UnmarshalBinary`, and `Validate`.
- roaring64/roaring64.go: equivalent `ReadFrom`, `UnmarshalBinary`, and `Validate` for 64-bit bitmaps.
- README.md (the serialization example, around lines 250-272): sample code showing the required validate-after-deserialize pattern for untrusted input, with the explicit comment: "if buf is an untrusted source, you should validate the result".

The format specification is documented at https://github.com/RoaringBitmap/RoaringFormatSpec. The Go, Java, C and C++ implementations are binary compatible.

A special note on `FromBuffer` and `FromUnsafeBytes`: these are zero-copy entry points (for advanced users only). The resulting bitmap holds references into the caller-provided byte slice and uses copy-on-write. It is the caller's responsibility to ensure that the input slice is not modified and remains valid for the lifetime of the bitmap (and of any bitmap derived from it). Reports that mutate the backing slice after the fact, or that free/reuse it while the bitmap is still alive, are describing documented misuse, not a library bug.

The fuzzing harnesses (`FuzzSmat`, `FuzzSerializationBuffer`, `FuzzSerializationStream`, and the corpus under `testdata/fuzz/`) and the property tests exist precisely to ensure the deserializers and the validator behave correctly under adversarial input. A report that treats "deserializing attacker-controlled bytes and then using the result without calling `Validate()`" as a bug in the deserializer is a misunderstanding of the stated API contract.

When triaging such reports, point to the validation requirement in the function documentation and the README example, and close as "not a bug / user error / documented behavior."

## Building, Testing, and Conventions

- This is a pure-Go library; there is no cgo. Use the standard Go toolchain.
- Run the test suite with `go test ./...` (the root package and the `roaring64` subpackage both have extensive tests).
- The `Makefile` exposes `make unconvert`, which runs `go tool unconvert -apply ./...` to remove unnecessary type conversions. Run it before proposing changes that touch type conversions.
- The root package implements 32-bit bitmaps; `roaring64/` implements 64-bit bitmaps and should be kept behaviorally consistent with the root package.
- Architecture-specific files exist for performance (e.g., `popcnt_amd64.s`, `setutil_arm64.s`, with `_generic.go` and `_compat.go` fallbacks). Any change to one path must be mirrored in the generic fallback so all build targets stay correct.
- Keep new code consistent with the surrounding style: match existing naming, error handling, and comment density. Public API changes must update doc comments and, where relevant, the README.
