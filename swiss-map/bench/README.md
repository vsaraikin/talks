# Swiss Map Benchmarks

Old map (Go 1.23) vs Swiss map (Go 1.24+).

## Setup

```bash
go install golang.org/dl/go1.23.0@latest
go1.23.0 download
```

## Run

```bash
# Swiss map (current Go)
go test -bench=. -benchmem -count=5 | tee swiss.txt

# Old map (Go 1.23) — needs go.mod downgrade
cp go.mod go.mod.bak
echo -e "module bench\n\ngo 1.23.0" > go.mod
go1.23.0 test -bench=. -benchmem -count=5 | tee old.txt
cp go.mod.bak go.mod && rm go.mod.bak
```

## Compare

```bash
go install golang.org/x/perf/cmd/benchstat@latest
benchstat old.txt swiss.txt
```

## What's tested

| Benchmark | What it measures |
|-----------|-----------------|
| Insert | `make(map, hint)` + fill N keys |
| InsertNoHint | `make(map)` + fill N keys (no pre-alloc) |
| LookupHit | read all existing keys |
| LookupMiss | read keys that don't exist |
| Delete | insert + delete all keys |
| Iterate | `for k, v := range m` |

Each benchmark runs at 3 sizes: 100, 10K, 1M keys.
