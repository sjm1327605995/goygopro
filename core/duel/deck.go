package duel

type Deck struct {
	Main  []*CardDataC
	Extra []*CardDataC
	Side  []*CardDataC
}

func (d *Deck) Clear() {
	d.Side = d.Side[0:0]
	d.Extra = d.Extra[0:0]
	d.Main = d.Main[0:0]
}
