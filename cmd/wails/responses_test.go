package main

import (
	"bytes"
	"testing"
)

// TestResponseEncodings locks down the CTOS_RESPONSE byte layouts. These
// functions are the single authoritative implementation of every variable
// length response the frontend can produce; the JavaScript side only sends
// semantic parameters via App.Respond*.
func TestResponseEncodings(t *testing.T) {
	cases := []struct {
		name string
		got  []byte
		want []byte
	}{
		{
			name: "SelectCard: count byte + one byte per index",
			got:  encodeSelectCardResponse([]int32{2, 0, 1}),
			want: []byte{3, 2, 0, 1},
		},
		{
			name: "SelectCard: empty selection",
			got:  encodeSelectCardResponse(nil),
			want: []byte{0},
		},
		{
			name: "SelectUnselect: count=1 + combined index",
			got:  encodeSelectUnselectResponse(1),
			want: []byte{1, 1},
		},
		{
			name: "SelectUnselect: index 0",
			got:  encodeSelectUnselectResponse(0),
			want: []byte{1, 0},
		},
		{
			name: "Counter: uint16 LE per card",
			got:  encodeCounterResponse([]int32{2, 0, 1}),
			want: []byte{2, 0, 0, 0, 1, 0},
		},
		{
			name: "Counter: >255 counter count uses full uint16",
			got:  encodeCounterResponse([]int32{300}),
			want: []byte{300 & 0xff, 300 >> 8},
		},
		{
			name: "SelectSum: total byte + select-list indices",
			got:  encodeSelectSumResponse(2, []int32{0}),
			want: []byte{2, 0},
		},
		{
			name: "SelectSum: must(1) + two picked",
			got:  encodeSelectSumResponse(3, []int32{0, 2}),
			want: []byte{3, 0, 2},
		},
		{
			name: "SelectPlace: player+loc+seq bytes",
			got:  encodeSelectPlaceResponse(0, 0x4, 2),
			want: []byte{0, 0x4, 2},
		},
		{
			name: "SelectPlace: opponent szone seq 6",
			got:  encodeSelectPlaceResponse(1, 0x8, 6),
			want: []byte{1, 0x8, 6},
		},
		{
			name: "SelectPlaces: two zones flat triples",
			got:  encodeSelectPlacesResponse([]int32{0, 0x4, 2, 1, 0x8, 3}),
			want: []byte{0, 0x4, 2, 1, 0x8, 3},
		},
		{
			name: "SortCard: permutation bytes",
			got:  encodeSortCardResponse([]int32{1, 2, 0}),
			want: []byte{1, 2, 0},
		},
		{
			name: "SortCardCancel: single 0xff",
			got:  encodeSortCardCancelResponse(),
			want: []byte{0xff},
		},
		{
			name: "IdleCmd: summon first card = (0<<16)|0",
			got:  encodeIdleCmdResponse(0, 0),
			want: []byte{0, 0, 0, 0},
		},
		{
			name: "IdleCmd: toBP = (0<<16)|6",
			got:  encodeIdleCmdResponse(0, 6),
			want: []byte{6, 0, 0, 0},
		},
		{
			name: "IdleCmd: idx 2 sset = (2<<16)|4 -> LE uint32",
			got:  encodeIdleCmdResponse(2, 4),
			want: []byte{4, 0, 2, 0},
		},
		{
			name: "BattleCmd: attack idx 3 = (3<<16)|1 -> 0x00030001 LE",
			got:  encodeBattleCmdResponse(3, 1),
			want: []byte{1, 0, 3, 0},
		},
		{
			name: "BattleCmd: toM2 = (0<<16)|2",
			got:  encodeBattleCmdResponse(0, 2),
			want: []byte{2, 0, 0, 0},
		},
		{
			name: "BattleCmd: activate idx 1 = (1<<16)|0 -> 0x00010000 LE",
			got:  encodeBattleCmdResponse(1, 0),
			want: []byte{0, 0, 1, 0},
		},
	}
	for _, tc := range cases {
		if !bytes.Equal(tc.got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}
