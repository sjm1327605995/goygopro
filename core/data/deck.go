package data

type Deck struct {
	Main  []*CardDataC
	Extra []*CardDataC
	Side  []*CardDataC
}

func (deck *Deck) Clear() {
	deck.Side = deck.Side[0:0]
	deck.Extra = deck.Extra[0:0]
	deck.Main = deck.Main[0:0]
}
