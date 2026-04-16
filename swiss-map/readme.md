# Swiss Map in Go 1.24 — Presentation Notes

## Slide Plan (TOC)

| #  | Slide | Key point |
| -- | ----- | --------- |
| 1  | **Title** | Swiss Map in Go 1.24 |
| 2  | **Switzerland Map** | Visual hook |
| 3  | **Introduction** | Who I am, what you'll get out of this talk |
| 4  | **Why Should We Care?** | 10-15% CPU on maps, real money at scale |
| 5  | **Origin Story** | Google Zurich 2017, Abseil, CppCon talk |
| 6  | **Part I: The Old Map** | Section divider |
| 7  | **Hash Table: The Idea** | key → hash → bucket, O(1) lookup, collisions |
| 8  | **Old Map: Structure** | hmap → buckets → tophash/keys/values/overflow |
| 9  | **Collision Resolution: Chaining** | Overflow buckets, pointer chasing, cache misses |
| 10 | **Old Map: Why 4 Buckets?** | B=2 → 2^B=4. Slots vs buckets |
| 11 | **Old Map: How Many Buckets?** | B from load factor, power-of-two trick, growth |
| 12 | **Old Map: Hash → Bucket** | hash & (numBuckets-1) |
| 13 | **Old Map: Inside a Bucket** | tophash, separate key/value arrays |
| 14 | **Old Map: 9th Element → Overflow** | New heap bucket, chain starts |
| 15 | **Old Map: Tophash & CPU Caches** | Sequential compare, cache eviction |
| 16 | **Old Map: Evacuation** | Load factor 6.5/8, 2x memory peak |
| 17 | **Old Map: Summary of Problems** | 5 problems table |
| 18 | **Part II: Swiss Table** | Section divider |
| 19 | **Chaining vs Open Addressing** | Side-by-side comparison |
| 20 | **Swiss Map Structure** | Directory → Tables → Groups → Slots |
| 21 | **Hash Split: h1 and h2** | How hash navigates the structure |
| 22 | **Metadata: h2 + Occupancy Bit** | Where h2 lives, control word |
| 23 | **Group: The Unit of Search** | ctrl word + 8 slots, contiguous memory |
| 24 | **Slot Internals** | K+V together, >128 bytes → pointer |
| 25 | **Parallel Matching: The Idea** | Compare all 8 at once → bitmask |
| 26 | **Parallel Matching: How It Works** | Portable path + SIMD + platforms |
| 27 | **Insert: Hash the Key** | hash → h1 + h2 |
| 28 | **Insert: Select Table** | table_index = hash >> (64 - globalDepth) |
| 29 | **Insert: Select Group** | h1 & (numGroups - 1) |
| 30 | **Insert: Match h2** | Compare h2 against ctrl, find candidate |
| 31 | **Insert: Find Free Slot** | First 0x80 slot, write h2 + K\|V |
| 32 | **Probing: Triangular** | Triangular numbers, visits all groups |
| 33 | **Tombstones** | 0xFE, can't clear slot |
| 34 | **Small Map Optimization** | hint ≤ 8 → single group, no directory |
| 35 | **Growth: How It Starts** | make(map, 10), growth_left = 14 |
| 36 | **Growth: Filling Up** | growth_left → 0 |
| 37 | **Growth: Doubling Groups** | 2 → 4 → 8 → ... → 1024 |
| 38 | **Growth: The Split at 1024** | Split into two tables of 1024 |
| 39 | **Growth: global_depth** | num_tables = 2^global_depth |
| 40 | **Growth: Extendible Hashing** | Table entries, local_depth |
| 41 | **Iteration + Mutation** | Old table for order, new for values |
| 42 | **Benchmarks: Go 1.23 vs Go 1.24** | Our benchmark results, delete regression |
| 43 | **Benchmarks: Memory** | 63% less, Datadog 726→217 MiB |
| 44 | **Struct: Old vs New** | hmap/bmap vs Map/table/group code |
| 45 | **Summary: Old vs New** | Side-by-side comparison table |
| 46 | **Credits** | All contributors and sources |
| 47 | **Questions?** | Closing slide |

---

## 1. Title

**Swiss Map in Go 1.24**

---

## 2. Switzerland Map

Visual hook — a physical map of Switzerland. The name "Swiss Table" was the team's internal codename (Swiss army knife: compact, multifunctional).

---

## 3. Introduction

- **Vladimir Saraikin**, Software Engineer
- 5 years building software for trading — small and large institutions and funds
- Disclaimer: not a Computer Science hash table expert. I came to this because I needed to understand what changed in Go 1.24 and why it matters for the code I write every day.

What you'll get out of this talk:
1. Refresher on how a hash table works and how Go's old map was built
2. Deep dive into Swiss Tables and the tricks that make them fast
3. Benchmarks, tradeoffs, and gotchas you should know before upgrading

Disclaimer: topics are interconnected, so we might jump between them a bit.

---

## 4. Why Should We Care?

Map is one of the hottest data structures in any Go program:

- **10-15% of total CPU time** spent on map operations — Google fleet (Michael Pratt, Go team)
- **~4% of total CPU** — ByteDance services
- **2-3% fleet-wide CPU reduction** — what Google measured after Swiss tables shipped

At Google or ByteDance scale, 2-3% is real money: servers, electricity, datacenter space. Even at smaller scale, any speedup here is essentially free — you just bump the Go version.

**Sources:** Michael Pratt — GopherCon talk ([youtube](https://www.youtube.com/watch?v=aqtIM5AK9t4)) · ByteDance — [Go proposal #54766](https://github.com/golang/go/issues/54766)

---

## 5. Origin Story

Timeline:

- **2017** — Sam Benzaquen, Alkis Evlogimenos, Matt Kulukundis and Roman Perepelitsa at Google Zurich design Swiss Tables for C++
- **2018** — Open-sourced as part of the Abseil C++ library
- **2022** — Go proposal [#54766](https://github.com/golang/go/issues/54766) opened: "switch to Swiss Tables"
- **Feb 2025** — Go 1.24 ships with the new map

Matt Kulukundis's CppCon 2017 talk — "Designing a Fast, Efficient, Cache-friendly Hash Table, Step by Step" — is the origin. The name "Swiss Table" was the team's internal codename (Swiss army knife: compact, multifunctional). The C++ source called the type `ch_table` for "closed hashing" → CH → Swiss.

Path to Go: Google Abseil (C++) → community prototypes by YunHao Zhang, PJ Malloy, andy-wm-arthur → Peter Mattis's [`cockroachdb/swiss`](https://github.com/cockroachdb/swiss) → Michael Pratt integrates into the Go 1.24 runtime.

---

## 6. Part I: The Old Map (Section Divider)

Before we can appreciate what changed, we need to remember what was there. This section walks through the classic hash table, then the specific shape of Go's pre-1.24 map.

---

## 7. Hash Table: The Idea

A hash table is a key → value store with **O(1) expected** lookup, insert, and delete.

Mechanics:
1. Apply a hash function to the key → get a pseudo-random 64-bit number
2. `hash % numBuckets` → pick a bucket **(we'll talk about that a bit later)**
3. Store / look up the entry there

Because the hash function is deterministic, writes and reads always land in the same bucket. Because buckets are a fixed-size array, reaching any bucket is O(1).

The catch: two keys can hash to the same bucket. That's a **collision**. Every hash table needs a strategy to handle collisions.

---

## 8. Old Map: Structure

Show the `11-old-map` diagram — hmap struct with count, B, hash0, *buckets, *oldbuckets. Then zoom into a single bucket: tophash array, keys array, values array, *overflow pointer. This is the full picture before we drill into individual pieces.

---

## 9. Collision Resolution: Chaining

Separate Chaining (1953, Luhn) — on collision, the new entry links to the previous one via a pointer, forming a linked list. Go's old map used a hybrid: each bucket held 8 inline entries, then an `*overflow` pointer to another 8-entry bucket on the heap.

- Overflow nodes live at random heap addresses → **pointer chasing** → CPU cache misses (~100 cycles per hop)
- Extra allocation per overflow → GC pressure

---

## 10. Old Map: Why 4 Buckets?

Parameter `B` controls bucket count: `numBuckets = 2^B`.

When `B = 2`: `2^2 = 4` buckets. NOT 8 — the number 8 is slots-per-bucket, not the bucket count. These are two different things:

- **4 buckets** — how many "lanes" the map has (controlled by B)
- **8 slots per bucket** — how many KV-pairs fit in one bucket (hardcoded)

Total capacity before overflow: `4 × 8 = 32` slots. B starts at 0 (1 bucket) and grows as the map fills.

Why power-of-two? Because `hash % numBuckets` can be replaced with `hash & (numBuckets - 1)` — a single bitwise AND instruction instead of expensive integer division.

---

## 11. Old Map: How Many Buckets?

`numBuckets = 2^B`, `loadFactor = 6.5/bucket`.

Example: `make(map[string]string, 10)` → `10 / 6.5 = 1.54` → nearest power of 2 ≥ 2 → `B = 1`, 2 buckets, 16 slots capacity.

B grows with data: B=0 (1 bucket, 8 slots), B=1 (2, 16), B=2 (4, 32), B=3 (8, 64)...

When to grow: avg elements per bucket > 6.5 → B++ → 2x buckets → evacuation.

Why power of 2: `hash % numBuckets` → `hash & (numBuckets - 1)` — 1 AND instruction instead of expensive division.

**Why load factor matters:** Load factor is a tradeoff between speed and memory. More elements per bucket → more collisions → longer overflow chains → slower lookups. Fewer elements → more empty slots wasting memory. Go chose 6.5/8 = 81.25% — at this threshold, on average no more than ~1 overflow bucket per 4 primary buckets. Set it lower (e.g. 50%) and the map is faster but uses 2x memory. Set it higher (e.g. 95%) and overflow chains get long, degrading lookup to near-linear. 6.5 was chosen empirically by the Go team.

---

## 12. Old Map: Hash → Bucket Selection

Step by step:

1. You have a key, e.g. `"hello"`
2. Apply the hash function: `hash("hello") = 0x39CE5231` — deterministic 64-bit number
3. Pick bucket: `0x39CE5231 & 3 = 1` (because `numBuckets - 1 = 3 = 0b11`)
4. Go to bucket 1

Important: even a perfect hash function will eventually put two different keys into the same bucket. This is unavoidable — you're mapping an infinite key space into 4 slots. Collisions are not a bug of the hash function; they're a mathematical inevitability (pigeonhole principle).

---

## 13. Old Map: Inside a Bucket

Bucket 1 starts empty (8 slots). As keys arrive whose hash points to bucket 1, they fill up sequentially:

```
Bucket 1:
  tophash: [A3][7F][12][B9][5E][C1][3A][D4]   ← all 8 occupied
  keys:    [k0][k1][k2][k3][k4][k5][k6][k7]
  values:  [v0][v1][v2][v3][v4][v5][v6][v7]
  *overflow: nil
```

Three arrays inside every bucket:

- **tophash** — 8 bytes, each storing the **top 8 bits** of that key's hash. A quick 1-byte filter: on lookup, Go compares tophash first (cheap) and only if it matches does it compare the full key (could be a large string — expensive). This avoids unnecessary full key comparisons for 7 out of 8 slots on average.

- **keys** — 8 keys stored contiguously in their own array: `keys[0], keys[1], ..., keys[7]`.

- **values** — 8 values in a separate array: `values[0], values[1], ..., values[7]`.

Keys and values are in **separate** arrays, not interleaved `key0,val0,key1,val1,...`. Why? To avoid alignment padding. If key is `int8` (1 byte) and value is `int64` (8 bytes), interleaving would add 7 bytes of padding after every key. Separate arrays pack tightly with zero waste.

---

## 14. Old Map: 9th Element → Overflow

A 9th key hashes to bucket 1. All 8 slots are full. What happens?

Go allocates a **new bucket on the heap** — an overflow bucket. The original bucket's `*overflow` pointer now points to it:

```
Bucket 1 (original):                    Overflow bucket:
  tophash: [A3][7F][12][B9]...           tophash: [F1][00][00]...
  keys:    [k0][k1][k2]...              keys:    [k8][  ][  ]...
  values:  [v0][v1][v2]...              values:  [v8][  ][  ]...
  *overflow: 0x8f10 ────────────►       *overflow: nil
                                         addr: 0x8f10 (random heap)
```

The overflow bucket is a **separate heap allocation** at a random memory address. This is the root of the cache problem — the CPU has no way to predict where this data lives.

If we keep hitting bucket 1, we get a chain: bucket → overflow → overflow → overflow... Each hop is a pointer chase to a random address.

---

## 15. Old Map: Tophash & CPU Cache Lines

Inside a bucket, the lookup scans tophash entries **one at a time**:

```go
for i := 0; i < 8; i++ {
    if b.tophash[i] == top {     // compare byte #i
        if b.keys[i] == key {   // compare full key #i
            return b.values[i]
        }
    }
}
```

This is 8 separate compare instructions. Here's why that matters for the CPU:

**Cache lines**: When the CPU loads `tophash[0]` into a register, it doesn't load just 1 byte — it loads an entire **cache line** (typically 64 bytes). So all 8 tophash bytes and possibly all 8 keys get pulled into L1 cache at once. Sounds great!

**The problem**: each comparison is a separate instruction. The CPU is multitasking — between comparing `key[0]` and `key[1]`, another thread or process might run. That other process needs the same CPU registers and cache lines. So the CPU **evicts** your data from the register/cache to make room.

When your comparison resumes, `key[1]` needs to be **reloaded** from memory. What was supposed to be a fast L1 cache hit becomes another RAM fetch.

With 8 sequential comparisons per bucket, this eviction can happen up to 8 times. Each eviction + reload costs ~4-100 cycles depending on which cache level was evicted.

This is exactly the problem Swiss Tables solve with parallel matching: instead of 8 separate operations, one SIMD instruction compares all 8 at once — no window for eviction.

---

## 16. Old Map: Evacuation

Go tracks the **average** number of elements per bucket (the load factor). When it exceeds ~6.5 elements per bucket (81% of 8), evacuation triggers:

1. A new bucket array is allocated with **2× the buckets** (B increments by 1)
2. Data migrates **incrementally** — not all at once, but a few buckets per insert
3. During migration, both old and new bucket arrays exist in memory → **2× memory peak**
4. Once all data migrates, old array is garbage collected

**When evacuation does NOT fire**: if you have a bad hash function and all keys cluster into 1-2 buckets with long overflow chains, the *average* across ALL buckets might still be below 6.5. Bucket 1 has 50 elements (via overflow), buckets 0,2,3 have 2 each. Average = (50+2+2+2)/4 = 14. OK that would trigger. But with more buckets and milder skew, it's possible to have terrible performance in some buckets while the average looks fine.

**The incremental approach** is clever — it avoids a latency spike from copying millions of entries at once. But it requires maintaining two copies of the bucket array simultaneously, and every operation must check both old and new arrays during the migration window. This adds complexity and overhead to every single map operation.

---

## 17. Old Map: Summary of Problems

5 problems table: overflow chains (cache miss), sequential compare (eviction), expensive growth (2x peak), low load factor (81%), separate K/V arrays (poor locality).

---

## 18. Part II: Swiss Table (Section Divider)

Now we switch sides. Same `map[K]V` interface, completely different internals.

---

## 19. Chaining vs Open Addressing

Side-by-side comparison of both approaches. Now that we know chaining's problems from Part I, show open addressing as the alternative.

**Open Addressing** (1957, Peterson) — all keys live directly in one contiguous array. On collision, probe the next slot. No pointers, no separate allocations. CPU prefetcher loves contiguous memory. Delete is trickier — tombstones needed.

Swiss Table uses open addressing, heavily optimized for modern CPUs.

---

## 20. Swiss Map Structure

Before diving into details, show the big picture first. Directory → Tables → Groups → Slots. Three levels of indirection:

- **Directory** — an array of pointers (table entries) to tables. Its size is always `2^globalDepth`. The directory itself doesn't hold any data — it's just a routing layer that maps the top bits of the hash to the right table. Think of it like a phone book index: letters A-Z point you to the right page, but the actual entries are on those pages. The directory exists so that the map can grow one table at a time instead of rehashing everything.
- **Table** — up to 1024 groups, tracks `growthLeft`. Each table is an independent mini hash table. Tables grow independently of each other — when one is full, only that one splits or doubles. This is what gives Swiss map bounded growth latency.
- **Group** — 8 slots + 64-bit control word
- **Slot** — K+V stored together

---

## 21. Hash Split: h1 and h2

OK so we have this 3-level structure: Directory → Table → Group. But how does the map know which table and which group to go to? That's where the hash split comes in. The 64-bit hash gets sliced into parts, and each part navigates one level of the structure:

The 64-bit hash gets sliced:

- **Top N bits** → directory index (`N = globalDepth`): `table_index = hash >> (64 - globalDepth)`
- **h1 (57 bits)** → which group: `h1 & (numGroups - 1)`
- **h2 (7 bits)** → stored in the control word for fast matching

Why split? Each part has a different job. h1 needs to be large (57 bits) because it selects among potentially thousands of groups — more bits = better distribution = fewer collisions. h2 only needs 7 bits because it's just a quick filter inside one group of 8 slots — 1 in 128 chance of false positive is good enough to avoid 99% of expensive full key comparisons. If we used all 64 bits for group selection, we'd have nothing left for fast matching inside the group. If we used all 64 bits for matching, we couldn't pick a group.

All extractions are 1-3 CPU instructions (shifts and masks). No division — everything is a power of two.

---

## 22. Metadata: h2 + Occupancy Bit

So we know h2 is 7 bits used for fast matching. But where is it stored? Each slot in a group has exactly 1 byte of metadata. That byte packs two things: the 7-bit h2 value and a 1-bit flag telling us whether the slot is occupied.

Why does this matter? Because 8 slots per group = 8 metadata bytes = 8 bytes total. These 8 bytes are packed into a single `uint64` called the **control word**. And that's the thing we compare against in one SIMD/bit-trick operation — not the keys themselves. We're comparing 8 metadata bytes at once to find candidates, then only checking the full key for the few matches.

| Value | Meaning |
|-------|---------|
| `0x00-0x7F` | occupied — the 7-bit h2 is stored here |
| `0x80` | empty slot (high bit set = not occupied) |
| `0xFE` | tombstone (deleted, keep probing past me) |

The high bit distinguishes occupied from empty/tombstone: if bit 7 is 0, the slot has data. If bit 7 is 1, it's either empty or deleted.

---

## 23. Group: The Unit of Search

A group is the smallest unit the map searches at a time. It has a **control word** (8 metadata bytes packed into one `uint64`) and **8 slots** with K+V stored together. Groups lie in contiguous memory — no pointer chasing, CPU prefetcher works.

In the old map, when we arrived at a bucket, we compared tophash bytes one by one — 8 separate if statements. The group's control word is the same idea (metadata for quick filtering) but the crucial difference is: all 8 bytes are in one uint64, which means we can compare them all at once.

---

## 24. Slot Internals

Remember the old map stored keys and values in separate arrays? In Swiss map, each slot holds K+V **together** — better cache locality when you find the key.

Key or value ≤ 128 bytes → stored **directly** in the slot. Key or value > 128 bytes → stored as a **pointer** to the heap. When a table grows, a pointer is just 8 bytes to copy — the actual data stays in place.

---

## 25. Parallel Matching: The Idea

Remember the old map problem: 8 sequential tophash comparisons, each a separate `if` statement, with a window for context switch between every two. Can we do better?

Instead of 8 sequential comparisons, we compare h2 against all 8 control word bytes **simultaneously** and get a bitmask of matches.

The idea: replicate our target h2 into all 8 byte positions of a uint64. XOR that with the control word. Any matching byte becomes 0x00. Then use a bit trick to detect which bytes are zero → bitmask of candidates.

h2 is only 7 bits → 1/128 chance of false positive → still need to verify the full key. But 99% of slots are eliminated in one operation. This is the core speedup of Swiss Tables.

---

## 26. Parallel Matching: How It Works

OK but how do you actually compare 8 bytes at once? Two approaches exist. The first one uses pure math and works everywhere. The second uses **SIMD** — "Single Instruction, Multiple Data" — a CPU feature that lets one instruction operate on multiple values at once. Think of it as: instead of a cashier scanning items one by one, you have a scanner that scans the entire conveyor belt in one pass.

**Portable path** (any architecture, 5-6 instructions):
```go
replicated := uint64(h2) * 0x0101010101010101  // broadcast h2
xored      := controlWord ^ replicated         // match → 0x00
result     := (xored - 0x0101010101010101) &
              ^xored & 0x8080808080808080       // detect zero bytes
```
No loops, no branches. Credited to Jeff Dean.

**SIMD path** (x86 with `GOAMD64=v2`, 3 instructions, ~1 cycle): `VPBROADCASTB` + `VPCMPEQB` + `VPMOVMSKB` — same logic but hardware-accelerated.

| Platform | Path | Instructions |
|----------|------|-------------|
| x86 + `GOAMD64=v2` | SIMD | 3 (~1 cycle) |
| x86 default (v1) | Portable | 5-6 |
| ARM (Apple Silicon) | Portable | 5-6 |

ARM NEON not yet implemented — Apple Silicon uses portable path.

---

## 27. Insert: Hash the Key

OK so now we know all the building blocks: the 3-level structure (directory → table → group), the hash split (h1 + h2), the control word with metadata, and how parallel matching works. Let's put it all together and walk through what actually happens when you write `m["hello"] = "world"` — step by step.

`hash(key)` → split into h1 (57 bits, selects group) + h2 (7 bits, fast matching).

---

## 28. Insert: Select Table

`table_index = hash >> (64 - global_depth)` — use top N bits of hash to pick table from directory. Just 2-3 CPU instructions (subtraction + right shift).

---

## 29. Insert: Select Group

`group = h1 & (numGroups - 1)` — single bitwise AND because group count is power of two.

---

## 30. Insert: Match h2

Compare h2 against all 8 ctrl bytes via SIMD/bit-trick. If match found → compare full key → if equal, update value. If key differs → not a duplicate.

---

## 31. Insert: Find Free Slot

Find first slot with 0x80 (empty) → write h2 into ctrl byte, write K+V into slot. Group full? → probe next group (triangular probing).

---

## 32. Probing: Triangular

```
step 0 → group i
step 1 → group i + 1
step 2 → group i + 3
step 3 → group i + 6
step 4 → group i + 10
```

Triangular numbers. Visits ALL groups when count = 2^M. Load factor 87.5% guarantees empty space → probing always terminates fast.

Parking lot analogy: try first row → full → skip ahead → skip even further → park.

---

## 33. Tombstones

This is the price we pay for open addressing. In the old map with chaining, delete was simple — just unlink a node from the list. But with open addressing, delete is tricky.

Imagine: keys A, B, C all hash to group 0. A goes to slot 0, B to slot 1, C to slot 2. Now delete B. If we just clear slot 1 (mark it empty, `0x80`), what happens when we look up C? We hash C → group 0, check slot 0 (A, not a match), check slot 1 — it's empty! We stop and return "not found." But C is right there in slot 2. We just can't see it because the empty slot broke the probe chain.

Solution: instead of marking the slot empty, mark it as a **tombstone** (`0xFE`). A tombstone means: "this slot is available for new inserts, but don't stop probing here — there might be keys placed further down." Lookups skip past tombstones. Inserts can reuse them.

The downside: tombstones accumulate. A map with lots of insert/delete churn fills up with tombstones that waste space and slow down probing (more slots to skip). When tombstones exceed ~10% of the table, Go runs a **pruning pass** — rehashes the table in place, removing all tombstones — before considering growing. This is also why delete is ~58% slower in Swiss map vs old map in our benchmarks.

---

## 34. Small Map Optimization

`make(map[string]int)` or hint ≤ 8 → **just a single group**. No directory, no table, no tombstones. On 9th insert → evolves to full structure. Most maps in real code are tiny — Go optimizes for this.

This is the starting point for the growth story: you start with a single group, then grow into the full Directory → Table → Group structure.

---

## 35. Growth: How It Starts

`make(map[string]string, 10)` → 2 groups, `growth_left = 14`. As elements are inserted, growth_left decreases.

---

## 36. Growth: Filling Up

14 elements inserted → `growth_left = 0`. Next insert → table must grow: double the groups (2 → 4).

---

## 37. Growth: Doubling Groups

New table gets 4 groups. Data migrates, old table is GC'd. `new growth_left = 4 × 7 - 14 - 1 = 13`. Continues: 4 → 8 → 16 → ... → 1024.

---

## 38. Growth: The Split at 1024

At 1024 groups + growth_left = 0, table doesn't double — it **splits** into two tables of 1024 groups each. Max ~7K entries copied per split. Bounded latency.

---

## 39. Growth: global_depth

`num_tables = 2^global_depth`. Tracks how many times directory has doubled. Each table grows independently — only the splitting table pays the cost.

---

## 40. Growth: Extendible Hashing

How does directory stay power of 2 when only one table splits? Directory uses **table entries** (pointers). Multiple entries can point to the same physical table.

Each table has `local_depth` — snapshot of `global_depth` when created. If `local_depth < global_depth` → table has spare entries → directory doesn't need to grow.

Split rules:
1. `growLeft == 0` → table at load cap
2. `groups < 1024` → double groups
3. `groups == 1024` → split into two
4. `localDepth < globalDepth` → reuse directory slots
5. All `localDepth == globalDepth` → directory doubles first

---

## 41. Iteration + Mutation

In Go you can delete and insert elements right during `range` — `delete(m, k)` and `m[newKey] = v` inside the loop are both legal. In C++ this is undefined behavior. In Go it's by design.

The problem: when the map grows (table splits or doubles groups), keys move to new positions. The iterator was walking the old layout — slot 0, slot 1, slot 2... After growth, a key from slot 2 might end up in a completely different group. The iterator would either miss it or return it twice.

Solution: the iterator holds a reference to the **old table** (before growth) and walks it to determine visit order. But when it needs to return the **value** of a key, it looks it up in the **new table** — because that's where the current data lives.

- Old table → "which keys to visit and in what order"
- New table → "what is the current value of this key"

If a key was deleted — it's not in the new table, iterator skips it. If a value was updated — iterator returns the fresh value from the new table.

> *"The hardest part of the implementation."* — Michael Pratt

---

## 42. Benchmarks: Go 1.23 vs Go 1.24

Our own benchmarks (Apple M4 Pro, ARM64, portable path):

| Operation | Old (1.23) | Swiss (1.24) | Change |
|-----------|-----------|-------------|--------|
| Insert 10K (pre-alloc) | 226 µs | 120 µs | 47% faster |
| Lookup hit 10K | 80 µs | 72 µs | 10% faster |
| Lookup miss 10K | 211 µs | 77 µs | 64% faster |
| Iterate 10K | 55 µs | 47 µs | 15% faster |
| Delete 1M | 95 ms | 150 ms | 58% slower |

Delete is slower due to tombstone bookkeeping.

---

## 43. Benchmarks: Memory

| Metric | Old | Swiss |
|--------|-----|-------|
| Load factor | 81% | 87.5% |
| Memory overhead | baseline | ~63% less |
| Datadog 3.5M el. | 726 MiB | 217 MiB |
| Overflow allocs | many | zero |

Higher load factor + no overflow allocations = less GC work.

---

## 44. Struct: Old vs New

Side-by-side of Go runtime structs:

Old: `hmap` → `*buckets` → `bmap{tophash[8], keys[8], values[8], *overflow}` — separate K/V arrays, overflow pointer chasing.

New: `Map` → `dirPtr` → `table{growthLeft, localDepth, groups}` → `group{ctrl uint64, slots[8]{K,V}}` — K+V together, no overflow, three-level directory.

---

## 45. Summary: Old vs New

| | Old Map | Swiss Map |
|---|---|---|
| Collision | Chaining (overflow) | Open addressing |
| Matching | Sequential (1 by 1) | Parallel (all 8) |
| K-V layout | Separate arrays | Together |
| Growth | Evacuation | Extendible hashing |
| Load factor | 81% | 87.5% |
| Cache | Pointer chasing | Contiguous |
| Max growth | ∝ map size | ~7K entries |

---

## 46. Credits

**C++ Swiss Table:** Matt Kulukundis, Sam Benzaquen, Alkis Evlogimenos, Roman Perepelitsa (Google Zurich, 2017)
**SIMD matching insight:** credited to Jeff Dean
**Go prototypes:** YunHao Zhang, PJ Malloy, andy-wm-arthur
**CockroachDB library:** Peter Mattis (`github.com/cockroachdb/swiss`)
**Go 1.24 runtime integration:** Michael Pratt (Go team, Google)

**Sources:**
- Michael Pratt — [GopherCon talk](https://www.youtube.com/watch?v=aqtIM5AK9t4)
- Brian Boreham (Grafana Labs) — "Swiss Maps in Go" deep dive
- Pure Storage talk on Swiss tables
- Matt Kulukundis — CppCon 2017
- Oleg Kozyrev — Russian-language walkthrough
- Skill Issue — detailed Swiss Tables deep dive
- [Go blog: Faster Go maps with Swiss Tables](https://go.dev/blog/swisstable)

---

## 47. Questions?

Closing slide.
