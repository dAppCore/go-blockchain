# Blockchain TUI Dashboard Design

## Goal

A terminal dashboard for the Lethean Go node. Displays chain sync status and
provides a block explorer, all rendered through the core/cli Frame (bubbletea)
layout system. Runs as a standalone binary in go-blockchain.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Binary location | `go-blockchain/cmd/chain/` | Self-contained, fewer deps than core/cli embedding |
| Architecture | Model library (`tui/`) + thin main | Reusable models for Wails desktop and core/cli later |
| Data source | Go-native packages directly | Full node in the TUI binary, no separate daemon needed |
| Frame layout | `"HCF"` (header, content, footer) | Minimal for day-one scope; sidebars added with future panels |
| Day-one scope | Chain status + block explorer | Wallet, mining, P2P peers, identity/melt deferred |
| Identity model | SSH key (melt) separate from wallet mnemonic | Linked in account metadata but independent derivation |

## Package Structure

```
go-blockchain/
  tui/
    node.go               # Node wrapper — starts chain + P2P, feeds status updates
    status_model.go       # Chain sync status bar (FrameModel, header region)
    explorer_model.go     # Block list / block detail / tx detail (FrameModel, content)
    keyhints_model.go     # Context-sensitive key hints (Model, footer region)
  cmd/
    chain/
      main.go             # Thin wiring: create node, create models, run Frame
```

### Dependencies

**Added to go-blockchain/go.mod:**

- `forge.lthn.ai/core/cli` — Frame, FrameModel interface, lipgloss (transitive)

**Transitive (via core/cli):**

- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/lipgloss`

**Deferred (future panels):**

- `github.com/charmbracelet/melt` — identity panel (SSH key to mnemonic)

## Layout

```
┌─────────────────────────────────────────────────┐
│ Header: StatusModel                             │
│ ⛓ height 6,312 │ sync 100% │ diff 1.2M │ 4 peers │
├─────────────────────────────────────────────────┤
│ Content: ExplorerModel                          │
│                                                 │
│  Height  Hash (short)   Txs   Size    Time      │
│  6312    cb9d5455...     2    1.4 KB  12s ago   │
│  6311    a3f7b210...     1    0.8 KB  2m ago    │
│  6310    91c4e8d2...     3    2.1 KB  4m ago    │
│  > Enter on row → block detail view             │
│  > Enter on tx → tx detail view                 │
│  > Esc → back to list                           │
│                                                 │
├─────────────────────────────────────────────────┤
│ Footer: KeyHintsModel                           │
│ ↑/↓ select │ enter view │ esc back │ q quit    │
└─────────────────────────────────────────────────┘
```

## Data Flow

1. `Node` wrapper starts P2P sync and chain storage in a background goroutine.
2. `Node` exposes `Updates() <-chan NodeStatus` — pushes every ~2 seconds with
   height, sync percentage, difficulty, peer count, and tip timestamp.
3. `StatusModel` (FrameModel) subscribes via a `tea.Cmd` that reads from the
   channel and returns a `NodeStatusMsg` to bubbletea's update loop.
4. `ExplorerModel` (FrameModel) queries `chain.Chain` directly for block lists
   and detail views. Read-only SQLite queries, already implemented.
5. `KeyHintsModel` (plain Model) renders static hints per-view. The
   ExplorerModel updates a shared hint reference when navigating between views.

## Node Wrapper

```go
// tui/node.go

type NodeStatus struct {
    Height      uint64
    TopHash     types.Hash
    Difficulty  uint64
    PeerCount   int
    SyncPct     float64    // 0.0–100.0
    TipTime     time.Time
}

type Node struct {
    chain   *chain.Chain
    syncer  /* P2P sync handle */
    cfg     *config.ChainConfig
    updates chan NodeStatus
    done    chan struct{}
}

func NewNode(dataDir string, cfg *config.ChainConfig) (*Node, error)
func (n *Node) Start(ctx context.Context) error
func (n *Node) Stop()
func (n *Node) Updates() <-chan NodeStatus
func (n *Node) Chain() *chain.Chain
```

`Start()` opens the SQLite chain store, connects to seed peers via P2P (Levin
handshake + timed sync), and runs a ticker goroutine that computes `NodeStatus`
every 2 seconds. P2P sync writes blocks to the chain store as they arrive.

No new P2P or chain logic — this wraps existing `chain.Chain` and P2P sync
code.

## ExplorerModel

```go
// tui/explorer_model.go

type explorerView int
const (
    viewBlockList explorerView = iota
    viewBlockDetail
    viewTxDetail
)

type ExplorerModel struct {
    chain    *chain.Chain
    view     explorerView
    cursor   int
    offset   uint64       // top block height shown
    block    *types.Block
    txIndex  int
    tx       *types.Transaction
    width    int
    height   int
}
```

### Navigation

ExplorerModel manages its own internal view stack (not Frame.Navigate — that is
reserved for switching between major panels like wallet/mining in future):

| View | Key | Action |
|------|-----|--------|
| Block list | `↑/↓` | Move cursor |
| Block list | `PgUp/PgDn` | Scroll by page |
| Block list | `Home` | Jump to chain tip |
| Block list | `Enter` | Open block detail |
| Block detail | `↑/↓` | Select transaction |
| Block detail | `Enter` | Open tx detail |
| Block detail | `Esc` | Back to block list |
| Tx detail | `Esc` | Back to block detail |

### Rendering

Plain text tables using lipgloss for alignment and styling. No external table
library — `fmt.Sprintf` with column widths derived from terminal width. Keeps
dependencies minimal.

Block queries use `chain.Chain.BlockByHeight()` and `chain.Chain.BlockByHash()`,
both already implemented and tested.

## cmd/chain/main.go

```go
func main() {
    dataDir := flag.String("data-dir", defaultDataDir(), "blockchain data directory")
    seeds := flag.String("seeds", "seeds.lthn.io:36940", "comma-separated seed peers")
    flag.Parse()

    cfg := config.Mainnet
    node, err := tui.NewNode(*dataDir, &cfg)
    // handle err ...

    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    node.Start(ctx)
    defer node.Stop()

    status := tui.NewStatusModel(node)
    explorer := tui.NewExplorerModel(node.Chain())
    hints := tui.NewKeyHintsModel()

    frame := cli.NewFrame("HCF")
    frame.Header(status)
    frame.Content(explorer)
    frame.Footer(hints)
    frame.Run()
}
```

~30 lines. The `tui/` package does all the work.

## Future Panels (Deferred)

These panels follow the same pattern — new `FrameModel` in `tui/`, wired into
Frame via `Navigate()` or layout expansion (HLCRF).

| Panel | Description | Trigger |
|-------|-------------|---------|
| Wallet | Balance, send/receive, address book | When wallet UI is needed |
| Mining | Hashrate, workers, found blocks | When mining dashboard is needed |
| P2P peers | Peer list, KDTree stats, latency | When peer visibility is needed |
| Identity | SSH key backup (melt), mnemonic display | When identity management is needed |
| Tx pool | Pending transactions, mempool stats | When mempool visibility is needed |

### Identity Design (Future)

SSH key identity (via charmbracelet/melt) and wallet mnemonic (CryptoNote
Electrum 1626-word) remain separate key systems:

- **SSH key** = P2P network identity, node authentication, message signing
- **Wallet mnemonic** = financial keys (spend key, view key)
- **Link:** Account metadata stores both identities, associating the SSH
  public key fingerprint with the wallet's public address

Melt provides `ToMnemonic(ed25519.PrivateKey)` and
`FromMnemonic(string) ed25519.PrivateKey` for SSH key backup and restore
via BIP39 seed words.

## Testing

- `tui/` models are unit-testable: construct model, send messages, assert
  View() output
- `Node` wrapper tested with a mock chain store (existing test patterns)
- `cmd/chain/` is integration-only (requires CGo crypto build + network)
- ExplorerModel tested against an in-memory chain store seeded with test blocks
