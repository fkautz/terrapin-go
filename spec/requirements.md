# Terrapin (Go) Requirements Catalog

Requirement IDs for the traceability gate (`trace-check`, default Go dialect).
Each requirement is a behavior verified by exactly one tagged Go test (`// Verifies:
REQ-...` immediately above the `func Test...`), or a recorded waiver. Scope is the full
v0.3 surface: `v3.go` (G, TreeRoot, ManifestBytes, Identifier, IdentifierFromReader,
ParseManifest), `tree.go` (TreeBuilder, BuildFromReader, PersistedTree, validation),
and the `cmd/terrapin` CLI. Section refs point at the shared Terrapin spec.

## Primitive G (gitoid-sha256) — §3.0

### REQ-G-001 — G equals sha256("blob <len>\0" + data)
- Section: §3.0
- Keyword: MUST

### REQ-G-002 — G is 32 bytes and deterministic
- Section: §3.0
- Keyword: MUST

### REQ-G-003 — G binds length (differs from raw sha256)
- Section: §3.0
- Keyword: MUST

### REQ-G-004 — G correct across Block-boundary sizes
- Section: §3.0
- Keyword: MUST

### REQ-G-005 — G avalanche on single-bit change
- Section: §3.0
- Keyword: SHOULD

## Manifest encoding & ParseManifest — §5.1, §5.2

### REQ-MAN-001 — ManifestBytes exact shape, 4 lines, final LF
- Section: §5.1
- Keyword: MUST

### REQ-MAN-002 — manifest value "sha256" vs digest prefix "terrapin-sha256:"
- Section: §5.1
- Keyword: MUST

### REQ-MAN-003 — length is byte length; block_size literal 2097152
- Section: §5.1
- Keyword: MUST

### REQ-MAN-004 — ParseManifest accepts canonical, length 0 and max uint64, roundtrips
- Section: §5.2
- Keyword: MUST

### REQ-MAN-005 — ParseManifest accepts canonical and rejects representative non-canonical
- Section: §5.2
- Keyword: MUST

### REQ-MAN-006 — ParseManifest rejects spacing defects (ENC-3)
- Section: §5.2
- Keyword: MUST

### REQ-MAN-007 — ParseManifest rejects value defects (ENC-6,7,8)
- Section: §5.2
- Keyword: MUST

### REQ-MAN-008 — ParseManifest rejects structural defects (ENC-2,4,5,9)
- Section: §5.2
- Keyword: MUST

### REQ-MAN-009 — ParseManifest rejects, never normalizes
- Section: §5.2
- Keyword: SHOULD

## Reference TreeRoot — §4

### REQ-TR-001 — zero-data boundary tree vectors
- Section: §4.3
- Keyword: MUST

### REQ-TR-002 — algebraic recursion-boundary vectors
- Section: §4.3
- Keyword: MUST

### REQ-TR-003 — empty dataset root is G("")
- Section: §4.3
- Keyword: MUST

### REQ-TR-004 — multi-block root equals G over concatenated leaf hashes
- Section: §4.3
- Keyword: MUST

### REQ-TR-005 — single leaf is the bare leaf, not G(leaf)
- Section: §4.3
- Keyword: MUST

### REQ-TR-006 — block order is significant
- Section: §4.3
- Keyword: MUST

### REQ-TR-007 — TreeRoot avalanche on single-byte change
- Section: §4.3
- Keyword: SHOULD

## Identifier — §5.3, §8

### REQ-ID-001 — explicit golden identifiers
- Section: §5.3
- Keyword: MUST

### REQ-ID-002 — identifier zero-data vectors
- Section: §5.3
- Keyword: MUST

### REQ-ID-003 — Identifier == G(ManifestBytes(len, TreeRoot))
- Section: §5.3
- Keyword: MUST

### REQ-ID-004 — prefix terrapin-sha256: and 64 lowercase hex
- Section: §5.3
- Keyword: MUST

### REQ-ID-005 — identifier is G(manifest), not the bare tree root
- Section: §8
- Keyword: MUST

### REQ-ID-006 — length is committed (same root, different length, different id)
- Section: §7
- Keyword: MUST

### REQ-ID-007 — distinct inputs yield distinct identifiers
- Section: §5.3
- Keyword: MUST

### REQ-ID-008 — identifier regression snapshot
- Section: §5.3
- Keyword: SHOULD

## Streaming IdentifierFromReader — §2.1

### REQ-SB-001 — reader identifier == in-memory (key sizes)
- Section: §2.1
- Keyword: MUST

### REQ-SB-002 — identifier stable under short-read chunking
- Section: §2.1
- Keyword: MUST

### REQ-SB-003 — empty reader yields identifier of empty
- Section: §2.1
- Keyword: MUST

### REQ-SB-004 — single short block matches in-memory
- Section: §2.1
- Keyword: MUST

### REQ-SB-005 — multi-block and boundary sizes match in-memory
- Section: §2.1
- Keyword: MUST

### REQ-SB-006 — reader error is surfaced, never a wrong identifier
- Section: §2.1
- Keyword: MUST

### REQ-SB-007 — streaming 2-layer matches the algebraic oracle
- Section: §4.3
- Keyword: SHOULD

### REQ-SB-008 — large (64 MiB) reader matches in-memory
- Section: §2.1
- Keyword: SHOULD

## Conformance

### REQ-CF-001 — load testdata/vectors-terrapin.json and verify all
- Section: §3
- Keyword: MUST

### REQ-CF-002 — boundary and §5.4 example vectors
- Section: §5.4
- Keyword: MUST

### REQ-CF-003 — frozen identifier corpus snapshot
- Section: §5.3
- Keyword: SHOULD

## Security / adversarial — §7

### REQ-SEC-001 — length reinterpretation is prevented
- Section: §7
- Keyword: MUST

### REQ-SEC-002 — bare tree root is not accepted as the identifier
- Section: §7
- Keyword: MUST

### REQ-SEC-003 — non-canonical manifest rejected at the parse boundary
- Section: §7
- Keyword: MUST

### REQ-SEC-004 — length framing detects truncation/short writes
- Section: §7
- Keyword: MUST

### REQ-SEC-005 — second-preimage resistance (foundational)
- Section: §7
- Keyword: SHOULD

## Spec worked example

### REQ-WE-001 — §5.4 single-block example (tree == G(dataset))
- Section: §5.4
- Keyword: MUST

## Property-based

### REQ-PR-001 — random data: streaming id == in-memory id
- Section: §2.1
- Keyword: SHOULD

### REQ-PR-002 — random chunking does not change the identifier
- Section: §2.1
- Keyword: SHOULD

### REQ-PR-003 — single-byte flip changes the identifier
- Section: §7
- Keyword: SHOULD

### REQ-PR-004 — random manifest roundtrip and mutation rejection
- Section: §5.2
- Keyword: SHOULD

## derive_counts / offsets — §6

### REQ-DC-001 — DeriveCounts small sizes and exact-fit boundaries
- Section: §6
- Keyword: MUST

### REQ-DC-002 — DeriveCounts multi-layer, 1 PiB, and max uint64
- Section: §6
- Keyword: MUST

### REQ-DC-003 — DeriveCounts matches builder layer counts
- Section: §6
- Keyword: MUST

### REQ-OFF-001 — offsetsFromCounts alignment and totals
- Section: §6
- Keyword: MUST

## TreeBuilder / BuiltTree

### REQ-TB-001 — single leaf yields bare-leaf root and identifier
- Section: §4.3
- Keyword: MUST

### REQ-TB-002 — empty path yields G("") root
- Section: §4.3
- Keyword: MUST

### REQ-TB-003 — multi-layer structure for Fanout+1 leaves
- Section: §4.3
- Keyword: MUST

### REQ-TB-004 — internal consistency: layer relation and root
- Section: §4.3
- Keyword: MUST

### REQ-TB-005 — leaf count, order, length, and tree hex
- Section: §4.3
- Keyword: MUST

## Streaming build

### REQ-BR-001 — BuildFromReader identifier and length match in-memory
- Section: §2.1
- Keyword: MUST

## PersistedTree write/read

### REQ-PT-001 — write/read roundtrip, .blocks/.head format, reproducible
- Section: §6
- Keyword: MUST

### REQ-PT-002 — ReadTree rejects malformed or inconsistent headers
- Section: §6
- Keyword: MUST

## Validation — success — §6

### REQ-VAL-001 — whole, single-block, copy, idempotent validate
- Section: §6
- Keyword: MUST

### REQ-VAL-002 — range variants validate
- Section: §6
- Keyword: MUST

## Validation — failure — §6, §7

### REQ-VF-001 — tamper, slice independence, length mismatch, bounds
- Section: §6
- Keyword: MUST

### REQ-VF-002 — corrupt artifact (leaf, truncated, missing, head id) rejected
- Section: §6
- Keyword: MUST

### REQ-VF-003 — forged/different/swapped/single-leaf/empty cases rejected
- Section: §7
- Keyword: MUST

## cat (validate + stream) — §6

### REQ-CAT-001 — cat whole and ranges equal data slices, binary-safe
- Section: §6
- Keyword: MUST

### REQ-CAT-002 — cat emits no bytes past first failure; writer errors surface
- Section: §6
- Keyword: SHOULD

## Spec worked example

### REQ-WE-004 — PathBlocks reads one hash-file block per layer
- Section: §6
- Keyword: SHOULD

## CLI (black-box)

### REQ-CLI-001 — id prints identifier; missing file errors
- Section: §5.3
- Keyword: MUST

### REQ-CLI-002 — attest writes tree files; identifier equals id
- Section: §6
- Keyword: MUST

### REQ-CLI-003 — validate succeeds whole and range; out-of-bounds errors
- Section: §6
- Keyword: MUST

### REQ-CLI-004 — validate and cat on tamper exit non-zero
- Section: §6
- Keyword: MUST

### REQ-CLI-005 — cat range equals the data slice
- Section: §6
- Keyword: MUST

### REQ-CLI-006 — bad arguments and missing tree error
- Section: §6
- Keyword: MUST

### REQ-CLI-007 — validate enforces a trusted -identifier
- Section: §6
- Keyword: MUST
