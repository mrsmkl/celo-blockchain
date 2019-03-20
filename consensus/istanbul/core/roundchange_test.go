// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/istanbul"
	"github.com/ethereum/go-ethereum/consensus/istanbul/validator"
)

func TestRoundChangeSet(t *testing.T) {
	vset := validator.NewSet(generateValidators(4), istanbul.RoundRobin)
	rc := newRoundChangeSet(vset)

	view := &istanbul.View{
		Sequence: big.NewInt(1),
		Round:    big.NewInt(1),
	}
	r := &istanbul.Subject{
		View:   view,
		Digest: common.Hash{},
	}
	m, _ := Encode(r)

	// Test Add()
	// Add message from all validators
	for i, v := range vset.List() {
		msg := &message{
			Code:    msgRoundChange,
			Msg:     m,
			Address: v.Address(),
		}
		rc.Add(view.Round, msg)
		if rc.roundChanges[view.Round.Uint64()].Size() != i+1 {
			t.Errorf("the size of round change messages mismatch: have %v, want %v", rc.roundChanges[view.Round.Uint64()].Size(), i+1)
		}
	}

	// Add message again from all validators, but the size should be the same
	for _, v := range vset.List() {
		msg := &message{
			Code:    msgRoundChange,
			Msg:     m,
			Address: v.Address(),
		}
		rc.Add(view.Round, msg)
		if rc.roundChanges[view.Round.Uint64()].Size() != vset.Size() {
			t.Errorf("the size of round change messages mismatch: have %v, want %v", rc.roundChanges[view.Round.Uint64()].Size(), vset.Size())
		}
	}

	// Test MaxRound()
	for i := 0; i < 10; i++ {
		maxRound := rc.MaxRound(i)
		if i <= vset.Size() {
			if maxRound == nil || maxRound.Cmp(view.Round) != 0 {
				t.Errorf("max round mismatch: have %v, want %v", maxRound, view.Round)
			}
		} else if maxRound != nil {
			t.Errorf("max round mismatch: have %v, want nil", maxRound)
		}
	}

	// Test Clear()
	for i := int64(0); i < 2; i++ {
		rc.Clear(big.NewInt(i))
		if rc.roundChanges[view.Round.Uint64()].Size() != vset.Size() {
			t.Errorf("the size of round change messages mismatch: have %v, want %v", rc.roundChanges[view.Round.Uint64()].Size(), vset.Size())
		}
	}
	rc.Clear(big.NewInt(2))
	if rc.roundChanges[view.Round.Uint64()] != nil {
		t.Errorf("the change messages mismatch: have %v, want nil", rc.roundChanges[view.Round.Uint64()])
	}
}


// listen will consume messages from queue and deliver a message to core
// Need to drop certain messages in the first round
// Also need to drop all messaages to/from a after it prepares in round 2 (same as going offline)
func (t *testSystem) mockListen(a *testSystemBackend, b *testSystemBackend, c *testSystemBackend, d *testSystemBackend) {
	for {
		select {
		case <-t.quit:
			return
		case queuedMessage := <-t.queuedMessage:
			testLogger.Info("consuming a queue message...")
			go a.EventMux().Post(queuedMessage)
			go b.EventMux().Post(queuedMessage)
			go c.EventMux().Post(queuedMessage)
			go d.EventMux().Post(queuedMessage)
		}
	}
}

// The test of liveness failure should work as follows:
// Have four nodes: {A, B, C, D}
// In round 1
//	- Block X is pre-prepared (proposed)
//	- A and B prepare Block X by getting 3 prepare votes each (including their own) (thus locking on it)
//	- C and D prepare nil by only getting 0-2 prepare votes (including own votes) each before timeout
//	- A and B can send commit messages, but I believe that these messages should not reach C and D before the
//	  timeout occurs as IBFT has an optimization to count commit messages as prepare messages from that node.
//	- Assuming the round 1 is before GST, messages may not be received before a node times out.
//	- Only round 1 needs to be prior to GST for this attack to work.
// In round 2
//	- Round 2 can proceed after GST (aka all sent messages get received on time)
//	- Block Y is pre-prepared (proposed)
//	- {A, C, D} prepare and lock on Block Y (A is byzantine here b/c it unlocked from Block X)
//	- B is still locked on Block X. (The correct behaviour is to unlock here)
//	- A goes offline after preparing Block Y (does not send commit message)
//	- C and D can send commit messages for Block Y, but quorom will not be achieved.
// In round 3
//	- {C, D} are locked on Block Y
//	- {B}    are locked on Block X
//	- {A}    are offline (Byzantine)
// In round 3, no progress will be able to be made unless B could have unlocked in round 2.
// Run: `./build/env.sh go test -run TestLivenessFailure -v github.com/ethereum/go-ethereum/consensus/istanbul/core` to run this test.
func TestLivenessFailure(t *testing.T) {
	sys := NewTestSystemWithBackend(4, 1)
	// Generate conflicting proposals.
	// Currently test backends can't sign or gossip
	blockX := makeBlock(1)
	// blockY := makeBlock(2)

	for _, b := range sys.backends {
		b.engine.Start() // start Istanbul core
	}
	// TODO: hijack this listen to mess with messages
	go sys.mockListen(sys.backends[0], sys.backends[1], sys.backends[2], sys.backends[3])

	// Want to use messages rather than requests
	sys.backends[0].NewRequest(blockX)
	// sys.backends[1].NewRequest(blockX)

	<-time.After(1 * time.Second)

	// sys.backends[0].NewRequest(blockY)

	// Manually open and close b/c hijacking sys.listen
	for _, b := range sys.backends {
		b.engine.Stop() // start Istanbul core
	}
	close(sys.quit)
}