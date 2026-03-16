// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

package types

import "testing"

func TestTxOutToKey_TargetType_Good(t *testing.T) {
	var target TxOutTarget = TxOutToKey{Key: PublicKey{1}, MixAttr: 0}
	if target.TargetType() != TargetTypeToKey {
		t.Errorf("TargetType: got %d, want %d", target.TargetType(), TargetTypeToKey)
	}
}

func TestTxOutMultisig_TargetType_Good(t *testing.T) {
	var target TxOutTarget = TxOutMultisig{MinimumSigs: 2, Keys: []PublicKey{{1}, {2}}}
	if target.TargetType() != TargetTypeMultisig {
		t.Errorf("TargetType: got %d, want %d", target.TargetType(), TargetTypeMultisig)
	}
}

func TestTxOutHTLC_TargetType_Good(t *testing.T) {
	var target TxOutTarget = TxOutHTLC{
		Flags:      0,
		Expiration: 10080,
	}
	if target.TargetType() != TargetTypeHTLC {
		t.Errorf("TargetType: got %d, want %d", target.TargetType(), TargetTypeHTLC)
	}
}
