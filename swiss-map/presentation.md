---
marp: true
theme: default
paginate: true
style: |
  @import url('https://fonts.googleapis.com/css2?family=Roboto:wght@300;400;500;700&family=Roboto+Mono:wght@400;500&display=swap');

  :root {
    --bg: #0f172a;
    --bg-card: #1e293b;
    --text: #e2e8f0;
    --text-muted: #94a3b8;
    --accent: #2dd4bf;
    --accent2: #fbbf24;
    --accent3: #a78bfa;
    --green: #34d399;
    --red: #f87171;
    --yellow: #fbbf24;
  }

  section {
    background: var(--bg);
    color: var(--text);
    font-family: 'Roboto', 'Helvetica Neue', Arial, sans-serif;
    font-size: 26px;
    line-height: 1.5;
    padding: 40px 60px;
  }

  h1 {
    color: var(--accent);
    font-weight: 700;
    font-size: 1.8em;
    margin-bottom: 0.3em;
    border-bottom: 2px solid var(--accent);
    padding-bottom: 6px;
  }

  h2 { color: var(--accent3); font-weight: 500; font-size: 1.3em; margin-top: 0.2em; }
  h3 { color: var(--accent2); font-weight: 500; font-size: 1.05em; }
  strong { color: var(--accent2); }
  em { color: var(--accent); font-style: normal; }

  code {
    font-family: 'Roboto Mono', 'SF Mono', monospace;
    background: var(--bg-card);
    color: var(--green);
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 0.88em;
  }

  pre {
    background: #0c1322 !important;
    border: 1px solid #1e3a4f;
    border-radius: 8px;
    padding: 14px 18px !important;
    font-size: 0.75em;
    line-height: 1.5;
  }

  pre code {
    background: transparent !important;
    color: #cbd5e1 !important;
    padding: 0;
  }

  pre code .hljs-comment, pre code .hljs-meta { color: #5eead4 !important; }
  pre code .hljs-keyword, pre code .hljs-built_in, pre code .hljs-type { color: #c4b5fd !important; }
  pre code .hljs-string { color: #6ee7b7 !important; }
  pre code .hljs-number, pre code .hljs-literal { color: #fbbf24 !important; }
  pre code .hljs-title { color: #2dd4bf !important; }

  blockquote {
    border-left: 3px solid var(--accent);
    background: var(--bg-card);
    padding: 8px 14px;
    border-radius: 0 6px 6px 0;
    color: var(--text-muted);
    font-style: italic;
    margin: 10px 0;
    font-size: 0.92em;
  }

  table { font-size: 0.82em; border-collapse: collapse; width: 100%; }
  th { background: var(--bg-card); color: var(--accent); font-weight: 500; padding: 8px 14px; border-bottom: 2px solid var(--accent); text-align: left; }
  td { background: var(--bg); color: var(--text); padding: 7px 14px; border-bottom: 1px solid #1e3a4f; }
  tr:nth-child(even) td { background: var(--bg-card); }
  td:first-child { color: var(--accent2); font-weight: 500; }
  li { margin-bottom: 2px; }
  a { color: var(--accent); }
  img { max-height: 420px; }

  section.lead h1 { font-size: 2.2em; text-align: center; border: none; }
  section.lead h2 { text-align: center; color: var(--text-muted); font-weight: 300; }
---

<!-- _class: lead -->

# Swiss Map in Go 1.24

## Vladimir Saraikin

---

![bg contain](diagrams/switzerland-physical-map.jpg)

---

# Introduction

About me:

- Software developer for several trading/finance companies

Today's talk:

- What is a hash table and how Go's map worked before 1.24
- What changed with Swiss Tables and why it's faster
- Benchmarks and tradeoffs



---

# Why Should We Care?

Map operations as % of total CPU time:

- **Google fleet** — 10-15%
- **ByteDance** — ~4%
- **Go 1.24 savings** — 2-3% fleet-wide

---

# Origin Story

- **2017** — Google Zurich, Swiss Tables for C++
- **2018** — open-sourced in Abseil
- **2022** — Go proposal #54766
- **2025** — Go 1.24 ships with new map

---

<!-- _class: lead -->

# Part I: The Old Map

---

# Hash Table: The Idea

![Hash Table Basics](diagrams/06-hash-table-idea.png)

---

# Old Map: Structure

![Old Map Structure](diagrams/11-old-map.png)

---

# Collision Resolution: Chaining

![Chaining](diagrams/08a-chaining-only.png)

---

# Old Map: Why 4 Buckets?

![Why 4 Buckets](diagrams/11a-why-4-buckets.png)

---

# Old Map: How Many Buckets?

![Bucket Count](diagrams/11b-bucket-count.png)

---

# Old Map: Hash → Bucket

![Hash to Bucket](diagrams/06-hash-table-idea.png)

```go
bucketIndex := hash & (numBuckets - 1)
```

---

# Old Map: Inside a Bucket

![Bucket Internals](diagrams/11c-bucket-internals.png)

---

# Old Map: 9th Element → Overflow

![Overflow](diagrams/11d-overflow.png)

---

# Old Map: Tophash & CPU Cache Lines

![CPU Cache](diagrams/11f-cpu-cache.png)

---

# Old Map: Evacuation

![Evacuation](diagrams/11g-evacuation.png)

---

# Old Map: Summary of Problems

| #   | Problem                 | Why it hurts                                 |
| --- | ----------------------- | -------------------------------------------- |
| 1   | **Overflow chains**     | Pointer chasing → cache miss per hop         |
| 2   | **Sequential compare**  | 8 separate ops → context switch evicts cache |
| 3   | **Expensive growth**    | Evacuation copies all data, 2× memory peak   |
| 4   | **Low load factor**     | 81% → wastes memory, grows too early         |
| 5   | **Separate K/V arrays** | Poor locality within bucket                  |

---

<!-- _class: lead -->

# Part II: Swiss Table

---

# Collision Resolution: Chaining vs Open Addressing

![Collision Resolution Comparison](diagrams/08-collision-resolution.png)

---

# Swiss Map Structure

![Swiss Map Structure](diagrams/22-swiss-map-structure.png)

---

# Hash Split: h1 and h2

![Hash Split](diagrams/16-hash-split.png)

---

# Metadata: h2 + Occupancy Bit

![Metadata](diagrams/17-metadata.png)

---

# Group: The Unit of Search

![Groups](diagrams/18-groups.png)

---

# Slot Internals

![Slot Internals](diagrams/25-slot-internals.png)

---

# Parallel Matching: The Idea

![SIMD Matching](diagrams/19-simd-matching.png)

---

# Parallel Matching: How It Works

![Portable Path](diagrams/19a-portable-path.png)

| Platform            | Path     | Instructions |
| ------------------- | -------- | ------------ |
| x86 + `GOAMD64=v2`  | SIMD     | 3 (~1 cycle) |
| x86 default (`v1`)  | Portable | 5-6          |
| ARM (Apple Silicon) | Portable | 5-6          |

---

# Insert: Hash the Key

![Insert Step 1](diagrams/19b-insert-step1.png)

---

# Insert: Select Table

![Insert Step 1b](diagrams/19b-insert-step1b.png)

---

# Insert: Select Group

![Insert Step 2](diagrams/19b-insert-step2.png)

---

# Insert: Match h2

![Insert Step 3](diagrams/19b-insert-step3.png)

---

# Insert: Find Free Slot

![Insert Step 4](diagrams/19b-insert-step4.png)

---

# Probing: Triangular

![Probing](diagrams/20-probing.png)

---

# Tombstones

![Tombstones](diagrams/21-tombstones.png)

---

# Small Map Optimization

![Small Map](diagrams/24-small-map.png)

---

# Growth: How It Starts

![Growth Start](diagrams/23-growth-start.png)

---

# Growth: Filling Up

![Growth Filling](diagrams/23a-growth-filling.png)

---

# Growth: Doubling Groups

![Growth Doubling](diagrams/23b-growth-doubling.png)

---

# Growth: The Split at 1024

![Growth Split](diagrams/23c-growth-split.png)

---

# Growth: global_depth

![Global Depth](diagrams/23d-global-depth.png)

---

# Growth: Extendible Hashing

![Extendible Hashing](diagrams/23e-extendible-hashing.png)

---

# Iteration + Mutation

![Iteration](diagrams/26-iteration.png)

---

# Benchmarks: Go 1.23 vs Go 1.24

![Benchmarks](diagrams/27-benchmarks.png)

Delete slower — **tombstone bookkeeping**.

---

# Benchmarks: Memory

![Memory Growth](diagrams/28-memory-growth.png)

---

# Struct: Old vs New

![Struct Comparison](diagrams/29-struct-comparison.png)

---

# Summary: Old vs New

|                 | Old Map             | Swiss Map              |
| --------------- | ------------------- | ---------------------- |
| **Collision**   | Chaining (overflow) | Open addressing        |
| **Matching**    | Sequential (1 by 1) | **Parallel (all 8)**   |
| **K-V layout**  | Separate arrays     | **Together**           |
| **Growth**      | Evacuation          | **Extendible hashing** |
| **Load factor** | 81%                 | **87.5%**              |
| **Cache**       | Pointer chasing     | **Contiguous**         |

---

# Credits

**C++:** Matt Kulukundis, Sam Benzaquen, Alkis Evlogimenos, Jeff Dean
**Go:** Michael Pratt, Peter Mattis, YunHao Zhang, PJ Malloy

**Sources:** GopherCon (Pratt), Grafana (Boreham), Yandex Cloud, Oleg Kozyrev, Pure Storage, Skill Issue

---

<!-- _class: lead -->

# Questions?

---

# **TPF Labs**, Trading Solutions — [tpf-labs.com](https://tpf-labs.com)

![w:400](diagrams/qr-tpf-labs.png)
