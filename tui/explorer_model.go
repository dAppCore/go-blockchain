// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	cli "forge.lthn.ai/core/cli/pkg/cli"

	"forge.lthn.ai/core/go-blockchain/chain"
	"forge.lthn.ai/core/go-blockchain/types"
)

// Compile-time check: ExplorerModel implements cli.FrameModel.
var _ cli.FrameModel = (*ExplorerModel)(nil)

type explorerView int

const (
	viewBlockList   explorerView = iota
	viewBlockDetail
	viewTxDetail
)

// blockRow is a pre-fetched summary for the block list.
type blockRow struct {
	Height     uint64
	Hash       types.Hash
	TxCount    int
	Timestamp  uint64
	Difficulty uint64
}

// ExplorerModel provides block list, block detail, and tx detail views.
// It implements [cli.FrameModel] for the content region of the TUI dashboard.
type ExplorerModel struct {
	chain  *chain.Chain
	view   explorerView
	cursor int
	rows   []blockRow

	// Block detail state.
	block     *types.Block
	blockMeta *chain.BlockMeta
	txCursor  int

	// Tx detail state.
	tx     *types.Transaction
	txHash types.Hash

	width  int
	height int
}

// NewExplorerModel creates an ExplorerModel backed by the given chain.
func NewExplorerModel(c *chain.Chain) *ExplorerModel {
	m := &ExplorerModel{chain: c}
	m.loadBlocks()
	return m
}

// Init returns nil — block list is loaded synchronously in the constructor.
func (m *ExplorerModel) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages. KeyMsg drives navigation, NodeStatusMsg
// triggers a block list refresh, and WindowSizeMsg stores the terminal size.
func (m *ExplorerModel) Update(msg tea.Msg) (cli.FrameModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case NodeStatusMsg:
		m.loadBlocks()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *ExplorerModel) handleKey(msg tea.KeyMsg) (cli.FrameModel, tea.Cmd) {
	switch m.view {
	case viewBlockList:
		return m.handleBlockListKey(msg)
	case viewBlockDetail:
		return m.handleBlockDetailKey(msg)
	case viewTxDetail:
		return m.handleTxDetailKey(msg)
	}
	return m, nil
}

func (m *ExplorerModel) handleBlockListKey(msg tea.KeyMsg) (cli.FrameModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case tea.KeyPgUp:
		m.cursor -= pageSize(m.height)
		if m.cursor < 0 {
			m.cursor = 0
		}
	case tea.KeyPgDown:
		m.cursor += pageSize(m.height)
		if m.cursor >= len(m.rows) {
			m.cursor = len(m.rows) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
	case tea.KeyHome:
		m.cursor = 0
	case tea.KeyEnter:
		if len(m.rows) > 0 && m.cursor < len(m.rows) {
			row := m.rows[m.cursor]
			blk, meta, err := m.chain.GetBlockByHeight(row.Height)
			if err == nil {
				m.block = blk
				m.blockMeta = meta
				m.txCursor = 0
				m.view = viewBlockDetail
				return m, m.viewChangedCmd()
			}
		}
	}
	return m, nil
}

func (m *ExplorerModel) handleBlockDetailKey(msg tea.KeyMsg) (cli.FrameModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.txCursor > 0 {
			m.txCursor--
		}
	case tea.KeyDown:
		if m.block != nil && m.txCursor < len(m.block.TxHashes)-1 {
			m.txCursor++
		}
	case tea.KeyEnter:
		if m.block != nil && len(m.block.TxHashes) > 0 && m.txCursor < len(m.block.TxHashes) {
			txHash := m.block.TxHashes[m.txCursor]
			tx, _, err := m.chain.GetTransaction(txHash)
			if err == nil {
				m.tx = tx
				m.txHash = txHash
				m.view = viewTxDetail
				return m, m.viewChangedCmd()
			}
		}
	case tea.KeyEsc:
		m.view = viewBlockList
		return m, m.viewChangedCmd()
	}
	return m, nil
}

func (m *ExplorerModel) handleTxDetailKey(msg tea.KeyMsg) (cli.FrameModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.view = viewBlockDetail
		return m, m.viewChangedCmd()
	}
	return m, nil
}

// viewChangedCmd returns a command that emits a ViewChangedMsg with hints
// appropriate for the current view.
func (m *ExplorerModel) viewChangedCmd() tea.Cmd {
	var hints []string
	switch m.view {
	case viewBlockList:
		hints = []string{"↑/↓ select", "enter view", "q quit"}
	case viewBlockDetail:
		hints = []string{"↑/↓ select tx", "enter view tx", "esc back", "q quit"}
	case viewTxDetail:
		hints = []string{"esc back", "q quit"}
	}
	return func() tea.Msg { return ViewChangedMsg{Hints: hints} }
}

// View renders the current view, delegating to the appropriate sub-view.
func (m *ExplorerModel) View(width, height int) string {
	m.width = width
	m.height = height

	switch m.view {
	case viewBlockList:
		return m.viewBlockList()
	case viewBlockDetail:
		return m.viewBlockDetail()
	case viewTxDetail:
		return m.viewTxDetail()
	}
	return ""
}

func (m *ExplorerModel) viewBlockList() string {
	if len(m.rows) == 0 {
		return " no blocks \u2014 chain is empty"
	}

	var b strings.Builder

	// Header row.
	header := fmt.Sprintf("  %-8s %-18s %5s %12s %12s",
		"Height", "Hash", "Txs", "Difficulty", "Age")
	b.WriteString(header)
	b.WriteByte('\n')

	// Visible window centred on cursor.
	visibleRows := max(m.height-2, 1) // header + bottom margin

	start := max(m.cursor-visibleRows/2, 0)
	end := start + visibleRows
	if end > len(m.rows) {
		end = len(m.rows)
		start = max(end-visibleRows, 0)
	}

	for i := start; i < end; i++ {
		row := m.rows[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}

		hashShort := fmt.Sprintf("%x", row.Hash[:4]) + "..."
		age := formatAge(time.Unix(int64(row.Timestamp), 0))

		line := fmt.Sprintf("%s%-8d %-18s %5d %12s %12s",
			prefix, row.Height, hashShort, row.TxCount,
			formatDifficulty(row.Difficulty), age)

		if m.width > 0 && len(line) > m.width {
			line = line[:m.width]
		}
		b.WriteString(line)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}

	return b.String()
}

func (m *ExplorerModel) viewBlockDetail() string {
	if m.block == nil {
		return " no block selected"
	}

	var b strings.Builder
	meta := m.blockMeta
	blk := m.block

	b.WriteString(fmt.Sprintf(" Block %d\n", meta.Height))
	b.WriteString(fmt.Sprintf(" Hash:       %x\n", meta.Hash))
	b.WriteString(fmt.Sprintf(" Timestamp:  %s\n", time.Unix(int64(meta.Timestamp), 0).UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf(" Difficulty: %s\n", formatDifficulty(meta.Difficulty)))
	b.WriteString(fmt.Sprintf(" Version:    %d.%d\n", blk.MajorVersion, blk.MinorVersion))
	b.WriteString(fmt.Sprintf(" Nonce:      %d\n", blk.Nonce))
	b.WriteString(fmt.Sprintf(" Txs:        %d\n\n", len(blk.TxHashes)))

	if len(blk.TxHashes) == 0 {
		b.WriteString(" (coinbase only)")
	} else {
		b.WriteString(" Transactions:\n")
		for i, txHash := range blk.TxHashes {
			prefix := "  "
			if i == m.txCursor {
				prefix = "> "
			}
			b.WriteString(fmt.Sprintf(" %s%x\n", prefix, txHash[:8]))
		}
	}

	return b.String()
}

func (m *ExplorerModel) viewTxDetail() string {
	if m.tx == nil {
		return " no transaction selected"
	}

	var b strings.Builder
	tx := m.tx

	b.WriteString(" Transaction\n")
	b.WriteString(fmt.Sprintf(" Hash:    %x\n", m.txHash))
	b.WriteString(fmt.Sprintf(" Version: %d\n", tx.Version))
	b.WriteString(fmt.Sprintf(" Inputs:  %d\n", len(tx.Vin)))
	b.WriteString(fmt.Sprintf(" Outputs: %d\n\n", len(tx.Vout)))

	if len(tx.Vin) > 0 {
		b.WriteString(" Inputs:\n")
		for i, in := range tx.Vin {
			switch v := in.(type) {
			case types.TxInputGenesis:
				b.WriteString(fmt.Sprintf("  [%d] coinbase height=%d\n", i, v.Height))
			case types.TxInputToKey:
				b.WriteString(fmt.Sprintf("  [%d] to_key amount=%d key_image=%x\n", i, v.Amount, v.KeyImage[:4]))
			default:
				b.WriteString(fmt.Sprintf("  [%d] %T\n", i, v))
			}
		}
	}

	if len(tx.Vout) > 0 {
		b.WriteString("\n Outputs:\n")
		for i, out := range tx.Vout {
			switch v := out.(type) {
			case types.TxOutputBare:
				b.WriteString(fmt.Sprintf("  [%d] bare amount=%d key=%x\n", i, v.Amount, v.Target.Key[:4]))
			case types.TxOutputZarcanum:
				b.WriteString(fmt.Sprintf("  [%d] zarcanum stealth=%x\n", i, v.StealthAddress[:4]))
			default:
				b.WriteString(fmt.Sprintf("  [%d] %T\n", i, v))
			}
		}
	}

	return b.String()
}

// loadBlocks refreshes the block list from the chain store.
// Blocks are listed from newest (top) to oldest.
func (m *ExplorerModel) loadBlocks() {
	height, err := m.chain.Height()
	if err != nil || height == 0 {
		m.rows = nil
		return
	}

	// Show up to 1000 most recent blocks.
	count := min(int(height), 1000)

	rows := make([]blockRow, count)
	for i := range count {
		h := height - 1 - uint64(i)
		blk, meta, err := m.chain.GetBlockByHeight(h)
		if err != nil {
			continue
		}
		rows[i] = blockRow{
			Height:     meta.Height,
			Hash:       meta.Hash,
			TxCount:    len(blk.TxHashes) + 1, // +1 for miner tx
			Timestamp:  meta.Timestamp,
			Difficulty: meta.Difficulty,
		}
	}
	m.rows = rows
}

// pageSize returns the number of rows to jump for page up/down.
func pageSize(height int) int {
	return max(height-3, 1)
}
