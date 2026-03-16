// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package chain

import (
	"log"

	coreerr "forge.lthn.ai/core/go-log"

	"forge.lthn.ai/core/go-blockchain/p2p"
	levinpkg "forge.lthn.ai/core/go-p2p/node/levin"
)

// LevinP2PConn adapts a Levin connection to the P2PConnection interface.
type LevinP2PConn struct {
	conn       *levinpkg.Connection
	peerHeight uint64
	localSync  p2p.CoreSyncData
}

// NewLevinP2PConn wraps a Levin connection for P2P sync.
// peerHeight is obtained from the handshake CoreSyncData.
// localSync is our local sync state sent in timed_sync responses.
func NewLevinP2PConn(conn *levinpkg.Connection, peerHeight uint64, localSync p2p.CoreSyncData) *LevinP2PConn {
	return &LevinP2PConn{conn: conn, peerHeight: peerHeight, localSync: localSync}
}

func (c *LevinP2PConn) PeerHeight() uint64 { return c.peerHeight }

// handleMessage processes non-target messages received while waiting for
// a specific response. It replies to timed_sync requests to keep the
// connection alive.
func (c *LevinP2PConn) handleMessage(hdr levinpkg.Header, data []byte) error {
	if hdr.Command == p2p.CommandTimedSync && hdr.ExpectResponse {
		// Respond to keep-alive. The daemon expects a timed_sync
		// response with our payload_data.
		resp := p2p.TimedSyncRequest{PayloadData: c.localSync}
		payload, err := resp.Encode()
		if err != nil {
			return coreerr.E("LevinP2PConn.handleMessage", "encode timed_sync response", err)
		}
		if err := c.conn.WriteResponse(p2p.CommandTimedSync, payload, levinpkg.ReturnOK); err != nil {
			return coreerr.E("LevinP2PConn.handleMessage", "write timed_sync response", err)
		}
		log.Printf("p2p: responded to timed_sync")
		return nil
	}
	// Silently skip other messages (new_block notifications, etc.)
	return nil
}

func (c *LevinP2PConn) RequestChain(blockIDs [][]byte) (uint64, [][]byte, error) {
	req := p2p.RequestChain{BlockIDs: blockIDs}
	payload, err := req.Encode()
	if err != nil {
		return 0, nil, coreerr.E("LevinP2PConn.RequestChain", "encode request_chain", err)
	}

	// Send as notification (expectResponse=false) per CryptoNote protocol.
	if err := c.conn.WritePacket(p2p.CommandRequestChain, payload, false); err != nil {
		return 0, nil, coreerr.E("LevinP2PConn.RequestChain", "write request_chain", err)
	}

	// Read until we get RESPONSE_CHAIN_ENTRY.
	for {
		hdr, data, err := c.conn.ReadPacket()
		if err != nil {
			return 0, nil, coreerr.E("LevinP2PConn.RequestChain", "read response_chain", err)
		}
		if hdr.Command == p2p.CommandResponseChain {
			var resp p2p.ResponseChainEntry
			if err := resp.Decode(data); err != nil {
				return 0, nil, coreerr.E("LevinP2PConn.RequestChain", "decode response_chain", err)
			}
			return resp.StartHeight, resp.BlockIDs, nil
		}
		if err := c.handleMessage(hdr, data); err != nil {
			return 0, nil, err
		}
	}
}

func (c *LevinP2PConn) RequestObjects(blockHashes [][]byte) ([]BlockBlobEntry, error) {
	req := p2p.RequestGetObjects{Blocks: blockHashes}
	payload, err := req.Encode()
	if err != nil {
		return nil, coreerr.E("LevinP2PConn.RequestObjects", "encode request_get_objects", err)
	}

	if err := c.conn.WritePacket(p2p.CommandRequestObjects, payload, false); err != nil {
		return nil, coreerr.E("LevinP2PConn.RequestObjects", "write request_get_objects", err)
	}

	// Read until we get RESPONSE_GET_OBJECTS.
	for {
		hdr, data, err := c.conn.ReadPacket()
		if err != nil {
			return nil, coreerr.E("LevinP2PConn.RequestObjects", "read response_get_objects", err)
		}
		if hdr.Command == p2p.CommandResponseObjects {
			var resp p2p.ResponseGetObjects
			if err := resp.Decode(data); err != nil {
				return nil, coreerr.E("LevinP2PConn.RequestObjects", "decode response_get_objects", err)
			}
			entries := make([]BlockBlobEntry, len(resp.Blocks))
			for i, b := range resp.Blocks {
				entries[i] = BlockBlobEntry{Block: b.Block, Txs: b.Txs}
			}
			return entries, nil
		}
		if err := c.handleMessage(hdr, data); err != nil {
			return nil, err
		}
	}
}
