// Copyright 2012 Aaron Jacobs. All Rights Reserved.
// Author: aaronjjacobs@gmail.com (Aaron Jacobs)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmac

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"math"
)

func padBlock(block []byte) []byte {
	blockLen := len(block)
	if blockLen == 16 {
		return block
	}

	result := make([]byte, 16)
	copy(result, block)
	result[blockLen] = 0x80

	return result
}

type cmacHash struct {
	// An AES cipher configured with the original key.
	ciph cipher.Block

	// Generated sub-keys.
	k1 []byte
	k2 []byte

	// Data that has been seen since the last block was disposed of (i.e. since
	// we finished an iteration of the for loop in RFC 4493's AES-CMAC algorithm
	// and were sure we were going into a new one).
	data []byte

	// The current value of X, as defined in the AES-CMAC algorithm in RFC 4493.
	// Initially this is a 128-bit zero, and it is updated with the current block
	// when we're sure it's not the last one.
	x []byte
}

func (h *cmacHash) Write(p []byte) (n int, err error) {
	// Consume the data.
	n = len(p)
	h.data = append(h.data, p...)

	// Consume any blocks that we're sure aren't the last.
	blocksToProcess := len(h.data) / 16
	if blocksToProcess > 0 && len(h.data) % 16 == 0 {
		blocksToProcess--
	}

	for i := 0; i < blocksToProcess; i++ {
		block := h.data[16*i:16*(i+1)]
		y := xor(h.x, block)
		h.ciph.Encrypt(h.x, y)
	}

	h.data = h.data[16*blocksToProcess:]

	return
}

// Given a 128-bit key and a message, return a MAC that can be used to validate
// the input message. This is the AES-CMAC function of RFC 4493.
func generateCmac(key []byte, msg []byte) ([]byte, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("generateCmac requires a 16-byte key.")
	}

	msgLen := len(msg)

	// Generate subkeys.
	k1, k2 := generateSubkey(key)

	// Calculate the number of rounds.
	lastBlockComplete := false
	n := uint64(math.Ceil(float64(msgLen) / 16))
	if n == 0 {
		n = 1
	} else {
		if msgLen % 16 == 0 {
			lastBlockComplete = true
		}
	}

	// Calculate M_last.
	lastBlock := msg[16*(n-1):]
	var mLast []byte
	if lastBlockComplete {
		mLast = xor(lastBlock, k1)
	} else {
		mLast = xor(padBlock(lastBlock), k2)
	}

	// Create a cipher.
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %v", err)
	}

	// Run the rounds.
	x := make([]byte, 16)
	for i := uint64(0); i < n-1; i++ {
		block := msg[16*i:16*(i+1)]
		y := xor(x, block)
		c.Encrypt(x, y)
	}

	y := xor(mLast, x)
	result := make([]byte, 16)
	c.Encrypt(result, y)

	return result, nil
}
