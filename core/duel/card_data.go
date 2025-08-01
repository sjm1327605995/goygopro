package duel

import "github.com/sjm1327605995/goygopro/ocgcore"

type CardDataC struct {
	Ot       uint32
	Category int64
	ocgcore.CardData
}
type CardString struct {
	Name string
	Text string
	Desc [16]string
}
