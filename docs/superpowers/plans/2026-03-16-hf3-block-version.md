# HF3 Block Version Validation Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox syntax for tracking.

**Goal:** Add block major version validation to `consensus/block.go` so the Go node enforces the correct block version at every hardfork boundary (HF0 through HF4+). This satisfies the HF3 spec and also covers HF1's block version requirement (HF1 plan Task 10).

**Architecture:** Two unexported pure functions (`checkBlockVersion`, `expectedBlockMajorVersion`) in `consensus/block.go`, called from `ValidateBlock`. One new sentinel error in `consensus/errors.go`. No new dependencies, no storage, no CGo.

**Tech Stack:** Go 1.26, `go test -race`, stdlib `testing` + testify assertions

---

## File Map

### Modified files

| File | What changes |
|------|-------------|
| `consensus/errors.go` | Add `ErrBlockVersion` sentinel error to the block errors group. |
| `consensus/block.go` | Add `checkBlockVersion` and `expectedBlockMajorVersion` functions. Call `checkBlockVersion` from `ValidateBlock` before timestamp validation. |
| `consensus/block_test.go` | Add table-driven tests for `checkBlockVersion` and `expectedBlockMajorVersion` covering all hardfork boundaries on both mainnet and testnet fork schedules. |

---

## Task 1: Sentinel error + expectedBlockMajorVersion + checkBlockVersion

**Package:** `consensus/`
**Why:** The version lookup and check are pure functions with no side effects. Delivering them together with their tests in one task keeps the change atomic and reviewable.

### Step 1.1 — Add ErrBlockVersion sentinel error

- [ ] Edit `/home/claude/Code/core/go-blockchain/consensus/errors.go`

Add to the block errors group, after `ErrMinerTxProofs`:

```go
	ErrBlockVersion    = errors.New("consensus: invalid block major version for height")
```

### Step 1.2 — Write tests for expectedBlockMajorVersion and checkBlockVersion

- [ ] Append to `/home/claude/Code/core/go-blockchain/consensus/block_test.go`

```go
func TestExpectedBlockMajorVersion_Good(t *testing.T) {
	tests := []struct {
		name    string
		forks   []config.HardFork
		height  uint64
		want    uint8
	}{
		// --- Mainnet ---
		{
			name:   "mainnet/genesis",
			forks:  config.MainnetForks,
			height: 0,
			want:   config.BlockMajorVersionInitial, // 0
		},
		{
			name:   "mainnet/pre_HF1",
			forks:  config.MainnetForks,
			height: 5000,
			want:   config.BlockMajorVersionInitial, // 0
		},
		{
			name:   "mainnet/at_HF1_boundary",
			forks:  config.MainnetForks,
			height: 10080,
			want:   config.BlockMajorVersionInitial, // 0 (fork at height > 10080)
		},
		{
			name:   "mainnet/post_HF1",
			forks:  config.MainnetForks,
			height: 10081,
			want:   config.HF1BlockMajorVersion, // 1
		},
		{
			name:   "mainnet/well_past_HF1",
			forks:  config.MainnetForks,
			height: 100000,
			want:   config.HF1BlockMajorVersion, // 1 (HF3 not yet active)
		},

		// --- Testnet (HF3 active from genesis) ---
		{
			name:   "testnet/genesis",
			forks:  config.TestnetForks,
			height: 0,
			want:   config.HF3BlockMajorVersion, // 2 (HF3 at 0)
		},
		{
			name:   "testnet/pre_HF4",
			forks:  config.TestnetForks,
			height: 50,
			want:   config.HF3BlockMajorVersion, // 2 (HF4 at >100)
		},
		{
			name:   "testnet/post_HF4",
			forks:  config.TestnetForks,
			height: 101,
			want:   config.CurrentBlockMajorVersion, // 3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expectedBlockMajorVersion(tt.height, tt.forks)
			if got != tt.want {
				t.Errorf("expectedBlockMajorVersion(%d) = %d, want %d", tt.height, got, tt.want)
			}
		})
	}
}

func TestCheckBlockVersion_Good(t *testing.T) {
	// Correct version at each mainnet era.
	tests := []struct {
		name    string
		version uint8
		height  uint64
		forks   []config.HardFork
	}{
		{"mainnet/v0_pre_HF1", config.BlockMajorVersionInitial, 5000, config.MainnetForks},
		{"mainnet/v1_post_HF1", config.HF1BlockMajorVersion, 10081, config.MainnetForks},
		{"testnet/v2_genesis", config.HF3BlockMajorVersion, 0, config.TestnetForks},
		{"testnet/v3_post_HF4", config.CurrentBlockMajorVersion, 101, config.TestnetForks},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkBlockVersion(tt.version, tt.height, tt.forks)
			require.NoError(t, err)
		})
	}
}

func TestCheckBlockVersion_Bad(t *testing.T) {
	tests := []struct {
		name    string
		version uint8
		height  uint64
		forks   []config.HardFork
	}{
		{"mainnet/v1_pre_HF1", config.HF1BlockMajorVersion, 5000, config.MainnetForks},
		{"mainnet/v0_post_HF1", config.BlockMajorVersionInitial, 10081, config.MainnetForks},
		{"mainnet/v2_post_HF1", config.HF3BlockMajorVersion, 10081, config.MainnetForks},
		{"testnet/v1_genesis", config.HF1BlockMajorVersion, 0, config.TestnetForks},
		{"testnet/v2_post_HF4", config.HF3BlockMajorVersion, 101, config.TestnetForks},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkBlockVersion(tt.version, tt.height, tt.forks)
			assert.ErrorIs(t, err, ErrBlockVersion)
		})
	}
}

func TestCheckBlockVersion_Ugly(t *testing.T) {
	// Version 255 should never be valid at any height.
	err := checkBlockVersion(255, 0, config.MainnetForks)
	assert.ErrorIs(t, err, ErrBlockVersion)

	err = checkBlockVersion(255, 10081, config.MainnetForks)
	assert.ErrorIs(t, err, ErrBlockVersion)

	// Version 0 at the exact HF1 boundary (height 10080 — fork not yet active).
	err = checkBlockVersion(config.BlockMajorVersionInitial, 10080, config.MainnetForks)
	require.NoError(t, err)
}
```

### Step 1.3 — Run tests, verify FAIL

```bash
cd /home/claude/Code/core/go-blockchain && go test -race -run "TestExpectedBlockMajorVersion|TestCheckBlockVersion" ./consensus/...
```

**Expected:** Compilation error — `expectedBlockMajorVersion`, `checkBlockVersion`, and `ErrBlockVersion` do not exist yet.

### Step 1.4 — Implement expectedBlockMajorVersion and checkBlockVersion

- [ ] Edit `/home/claude/Code/core/go-blockchain/consensus/block.go`

Add after the `medianTimestamp` function, before `ValidateMinerTx`:

```go
// expectedBlockMajorVersion returns the required block major version for the
// given height based on the active hardfork schedule.
func expectedBlockMajorVersion(height uint64, forks []config.HardFork) uint8 {
	switch {
	case config.IsHardForkActive(forks, config.HF4Zarcanum, height):
		return config.CurrentBlockMajorVersion // 3
	case config.IsHardForkActive(forks, config.HF3, height):
		return config.HF3BlockMajorVersion // 2
	case config.IsHardForkActive(forks, config.HF1, height):
		return config.HF1BlockMajorVersion // 1
	default:
		return config.BlockMajorVersionInitial // 0
	}
}

// checkBlockVersion validates that the block's major version matches the
// expected version for its height in the hardfork schedule.
func checkBlockVersion(majorVersion uint8, height uint64, forks []config.HardFork) error {
	expected := expectedBlockMajorVersion(height, forks)
	if majorVersion != expected {
		return fmt.Errorf("%w: got %d, expected %d at height %d",
			ErrBlockVersion, majorVersion, expected, height)
	}
	return nil
}
```

### Step 1.5 — Wire checkBlockVersion into ValidateBlock

- [ ] Edit `/home/claude/Code/core/go-blockchain/consensus/block.go` — `ValidateBlock`

Add at the top of the function body, before the timestamp validation comment:

```go
	// Block major version check.
	if err := checkBlockVersion(blk.MajorVersion, height, forks); err != nil {
		return err
	}
```

### Step 1.6 — Run new tests, verify PASS

```bash
cd /home/claude/Code/core/go-blockchain && go test -race -run "TestExpectedBlockMajorVersion|TestCheckBlockVersion" ./consensus/...
```

**Expected:** All PASS.

### Step 1.7 — Run full consensus test suite

```bash
cd /home/claude/Code/core/go-blockchain && go test -race ./consensus/...
```

**Expected:** Existing `TestValidateBlock_Good` may FAIL because it uses `MajorVersion: 1` at height 100 (pre-HF1, where version 0 is expected). If so, fix the test block's `MajorVersion` to `0`. Repeat until all PASS.

**Likely fix** in `TestValidateBlock_Good` — change `MajorVersion: 1` to `MajorVersion: 0` (height 100 is pre-HF1 on mainnet):

```go
	blk := &types.Block{
		BlockHeader: types.BlockHeader{
			MajorVersion: 0, // height 100 is pre-HF1
			Timestamp:    now,
			Flags:        0,
		},
		MinerTx: *validMinerTx(height),
	}
```

Also update `TestValidateBlock_Bad_Timestamp` and `TestValidateBlock_Bad_MinerTx` if they use `MajorVersion: 1` at pre-HF1 heights.

### Step 1.8 — Run vet + mod tidy

```bash
cd /home/claude/Code/core/go-blockchain && go vet ./consensus/... && go mod tidy
```

**Expected:** Clean.

### Step 1.9 — Commit

```bash
cd /home/claude/Code/core/go-blockchain
git add consensus/errors.go consensus/block.go consensus/block_test.go
git commit -m "feat(consensus): validate block major version across all hardforks

Add checkBlockVersion and expectedBlockMajorVersion to enforce the correct
block major version at every hardfork boundary (v0 pre-HF1, v1 post-HF1,
v2 post-HF3, v3 post-HF4). This covers HF3's version gate and also
satisfies HF1 plan Task 10.

Co-Authored-By: Charon <charon@lethean.io>"
```

---

## Task 2: ValidateBlock integration — verify existing callers still work

**Package:** `consensus/`, `chain/`
**Why:** `ValidateBlock` gained a new early-return path. Callers in `chain/` (block storage and sync) must still compile and pass their tests.

### Step 2.1 — Run full test suite

```bash
cd /home/claude/Code/core/go-blockchain && go test -race ./...
```

**Expected:** All PASS. If any test in `chain/` or elsewhere constructs a `types.Block` with the wrong `MajorVersion` for its height, fix the test data.

### Step 2.2 — Run vet across entire module

```bash
cd /home/claude/Code/core/go-blockchain && go vet ./...
```

**Expected:** Clean.

### Step 2.3 — Commit (only if test fixes were needed)

```bash
cd /home/claude/Code/core/go-blockchain
git add -A
git commit -m "test: fix block MajorVersion in existing tests for version validation

Update test blocks to use the correct MajorVersion for their height
now that ValidateBlock enforces version checks.

Co-Authored-By: Charon <charon@lethean.io>"
```

If no fixes were needed, skip this commit.
