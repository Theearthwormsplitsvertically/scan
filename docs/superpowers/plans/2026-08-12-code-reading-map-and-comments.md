# Asset Agent Code Reading Map and Comments Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Chinese code-reading map and GoDoc-style comments that explain the current Agent foundation without changing runtime behavior.

**Architecture:** The new map is the navigation entry point: it explains runtime flow, package dependency direction, file responsibilities, and function-level call relationships. Source comments are added at package, exported-symbol, and key internal-function boundaries; behavior, signatures, and test fixtures remain unchanged.

**Tech Stack:** Markdown, Mermaid, Go 1.26.5, `gofmt`, `go vet`, and Go test suite.

## Global Constraints

- Do not modify Agent collection behavior, output schema, commands, or security boundaries.
- Documentation is Chinese; exported Go identifiers use GoDoc-style comments beginning with the identifier.
- Cover every production `.go` file; distinguish production code from `*_test.go` verification code.
- The map must describe only the current milestone, marking unimplemented capabilities as future work.
- Verify formatting, static checks, complete tests, and Linux AMD64 cross-build before commit.

---

### Task 1: Write the reading map

**Files:**
- Create: `docs/代码阅读地图.md`

**Interfaces:**
- Consumes: current `cmd/` and `internal/` production package layout.
- Produces: a reader-facing map with entry commands, calling layers, package dependencies, file/function matrix, test map, and recommended reading order.

- [ ] **Step 1: Derive the exact production-file inventory**

Run `rg --files -g '*.go' -g '!**/*_test.go'` and use that list as the map's file matrix.

- [ ] **Step 2: Add the architecture and dependency diagrams**

Use Mermaid diagrams for the CLI-to-runtime-to-collector flow and for the socket inode-to-process evidence chain. State that `watch`, service/package/container attribution, deep collection, scheduling, and retention are not implemented in this milestone.

- [ ] **Step 3: Add per-file and per-function responsibilities**

For every production file, list its types/functions, direct dependencies, callers, and responsibility. Include a separate test-file section explaining which behavior each test package protects.

- [ ] **Step 4: Review the map against the source inventory**

Confirm every production file appears once, every CLI command is explained, and no future feature is described as implemented.

### Task 2: Add source comments

**Files:**
- Modify: every production `*.go` file under `cmd/asset-agent/` and `internal/`

**Interfaces:**
- Consumes: existing code and behavior.
- Produces: Chinese comments for package purposes, exported APIs, data models, and key internal helpers.

- [ ] **Step 1: Add package and exported-symbol comments**

Comment each package, each exported type/function/method, and build-tag-specific runtime factory. Keep comments adjacent to declarations and begin GoDoc comments with the declaration name.

- [ ] **Step 2: Add internal-function comments at control boundaries**

Document command routing, file-root boundary checks, collector fallback rules, parsers, relationship correlation, timeout/panic isolation, and JSON normalization.

- [ ] **Step 3: Preserve behavior**

Do not alter function signatures, condition logic, literals, output field names, or tests. Apply only comments and the reading-map document.

### Task 3: Verify and commit

**Files:**
- Verify: all changed files

- [ ] **Step 1: Format and inspect the diff**

Run `go fmt ./...`, `git diff --check`, and inspect `git diff --stat`; the diff must only contain comments/documentation/formatting normalization.

- [ ] **Step 2: Run static checks and tests**

Run `go vet ./...` and `go test ./... -count=1`.

- [ ] **Step 3: Cross-build Linux artifact**

Run with `GOOS=linux`, `GOARCH=amd64`, and `CGO_ENABLED=0`: `go build -trimpath -ldflags '-s -w' -o dist/asset-agent-linux-amd64 ./cmd/asset-agent`.

- [ ] **Step 4: Commit**

```powershell
git add -- docs/代码阅读地图.md docs/superpowers/plans/2026-08-12-code-reading-map-and-comments.md cmd/asset-agent internal
git commit -m "docs: add agent code reading map and comments"
```
