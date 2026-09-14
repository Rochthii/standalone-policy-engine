# Dense Candidate and Collision Evidence — 2026-09-14

## Scope

`PERF-WORST-01` measures the in-memory `Engine.CheckPermission` path when every request must evaluate 10,000 matching candidates. It also closes the FNV-1a index correctness gap by retaining and matching the raw key at every trie level.

The two benchmark cases deliberately avoid an early `forbid` exit:

| Case | Candidate placement |
|---|---|
| `BenchmarkDenseCandidates_Global10000` | 10,000 global policies |
| `BenchmarkDenseCandidates_SameLeaf10000` | 10,000 policies at one subject/resource/action leaf |

## Reproduction

Environment: Windows/amd64, 13th Gen Intel(R) Core(TM) i7-13700H, Go `go1.26.4`.

```powershell
go test ./internal/engine ./tests
go test -run=^$ -bench='BenchmarkDenseCandidates_(Global|SameLeaf)10000$' -benchmem -benchtime=1s -count=3 ./tests
```

## Raw samples

| Benchmark | Sample 1 | Sample 2 | Sample 3 | Reported allocation data |
|---|---:|---:|---:|---|
| Global 10,000 | 378,955 ns/op | 374,917 ns/op | 372,470 ns/op | 27–33 B/op, 0 allocs/op |
| Same leaf 10,000 | 388,957 ns/op | 381,337 ns/op | 386,651 ns/op | 31–34 B/op, 0 allocs/op |

The benchmark reports zero allocations per operation, but not zero bytes per operation. The bytes are amortized pool/runtime activity; this evidence must not be restated as a universal zero-byte or full-path allocation claim.

## Collision correctness

Trie nodes are now held in hash buckets and carry their raw subject, resource, or action key. Lookup checks that raw key before collecting policies. `TestTrie_ForcedHashCollisionBucketMatchesRawKeys` constructs a collision bucket with distinct keys at every trie level and proves only the exact-key policy is returned.

This is a forced bucket test of the guard, not a claim that the test generated a natural 64-bit FNV collision.

## Remaining limitation

Global and same-leaf candidate lists are intentionally iterated to preserve forbid-overrides semantics. The measured density cases bound neither arbitrary policy sets nor the JWT, proof, gRPC, metrics, audit, database, or Odoo path. Those are the scope of `PERF-FULL-02` and `PERF-ODOO-03`.
