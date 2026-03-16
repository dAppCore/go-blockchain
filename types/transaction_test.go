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
