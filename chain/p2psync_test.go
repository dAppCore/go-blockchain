// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"context"
	"fmt"
	"testing"

	store "dappco.re/go/core/store"
	"dappco.re/go/core/blockchain/config"
	"github.com/stretchr/testify/require"
)

// mockP2PConn implements P2PConnection for testing.
type mockP2PConn struct {
	peerHeight uint64
	// blocks maps hash -> (blockBlob, txBlobs)
	blocks map[string]struct {
		blockBlob []byte
		txBlobs   [][]byte
	}
	chainHashes [][]byte

	// requestChainErr, if set, is returned by RequestChain.
	requestChainErr error
	// requestObjectsErr, if set, is returned by RequestObjects.
	requestObjectsErr error
}

func (m *mockP2PConn) PeerHeight() uint64 { return m.peerHeight }

func (m *mockP2PConn) RequestChain(blockIDs [][]byte) (startHeight uint64, hashes [][]byte, err error) {
	if m.requestChainErr != nil {
		return 0, nil, m.requestChainErr
	}
	return 0, m.chainHashes, nil
}

func (m *mockP2PConn) RequestObjects(blockHashes [][]byte) ([]BlockBlobEntry, error) {
	if m.requestObjectsErr != nil {
		return nil, m.requestObjectsErr
	}
	var entries []BlockBlobEntry
	for _, h := range blockHashes {
		key := string(h)
		if blk, ok := m.blocks[key]; ok {
			entries = append(entries, BlockBlobEntry{
				Block: blk.blockBlob,
				Txs:   blk.txBlobs,
			})
		}
	}
	return entries, nil
}

func TestP2PSync_EmptyChain(t *testing.T) {
	// Test that P2PSync with a mock that has no blocks is a no-op.
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	mock := &mockP2PConn{peerHeight: 0}

	opts := SyncOptions{Forks: config.TestnetForks}
	err = c.P2PSync(context.Background(), mock, opts)
	require.NoError(t, err)
}

func TestP2PSync_ContextCancellation(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	mock := &mockP2PConn{peerHeight: 100} // claims 100 blocks but returns none

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	opts := SyncOptions{Forks: config.TestnetForks}
	err = c.P2PSync(ctx, mock, opts)
	require.ErrorIs(t, err, context.Canceled)
}

func TestP2PSync_NoBlockIDs(t *testing.T) {
	// Peer claims a height but returns no block IDs from RequestChain.
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	mock := &mockP2PConn{
		peerHeight:  10,
		chainHashes: nil, // empty response
	}

	opts := SyncOptions{Forks: config.TestnetForks}
	err = c.P2PSync(context.Background(), mock, opts)
	require.NoError(t, err)
}

func TestP2PSync_RequestChainError(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	mock := &mockP2PConn{
		peerHeight:      10,
		requestChainErr: fmt.Errorf("connection reset"),
	}

	opts := SyncOptions{Forks: config.TestnetForks}
	err = c.P2PSync(context.Background(), mock, opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "request chain")
}

func TestP2PSync_RequestObjectsError(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()

	c := New(s)
	mock := &mockP2PConn{
		peerHeight:        10,
		chainHashes:       [][]byte{{0x01}},
		requestObjectsErr: fmt.Errorf("timeout"),
	}

	opts := SyncOptions{Forks: config.TestnetForks}
	err = c.P2PSync(context.Background(), mock, opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "request objects")
}
