package main

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"log/slog"
)

// Locations
const (
	LOCATION_DECK    = 0x01  //
	LOCATION_HAND    = 0x02  //
	LOCATION_MZONE   = 0x04  //
	LOCATION_SZONE   = 0x08  //
	LOCATION_GRAVE   = 0x10  //
	LOCATION_REMOVED = 0x20  //
	LOCATION_EXTRA   = 0x40  //
	LOCATION_OVERLAY = 0x80  //
	LOCATION_ONFIELD = 0x0c  //
	LOCATION_FZONE   = 0x100 //
	LOCATION_PZONE   = 0x200 //
)

// Positions
const (
	POS_FACEUP_ATTACK    = 0x1
	POS_FACEDOWN_ATTACK  = 0x2
	POS_FACEUP_DEFENSE   = 0x4
	POS_FACEDOWN_DEFENSE = 0x8
	POS_FACEUP           = 0x5
	POS_FACEDOWN         = 0xa
	POS_ATTACK           = 0x3
	POS_DEFENSE          = 0xc
)

func main() {
	sqlxDb, err := sqlx.Open("sqlite3", "E:\\YGOPro2\\cdb\\cards.cdb")
	if err != nil {
		panic(err)
	}
	dataManager := NewDataManager(sqlxDb)
	err = dataManager.ReadDB()
	if err != nil {
		panic(err)
	}
	err = ocgcore.Init(ocgcore.WithRootPath("E:\\Go\\gopath\\goygopro"),
		ocgcore.WithScriptDirectory("E:\\Go\\gopath\\src\\go-ygocore"),
		ocgcore.WithCardReader(func(cardId uint32, card *ocgcore.CardData) uint {
			cardData, has := dataManager.GetCardData(cardId)
			if has {
				card.Alias = cardData.Alias
				card.Setcode = cardData.Setcode
				card.Type = cardData.Type
				card.Attack = cardData.Attack
				card.Defense = cardData.Defense
				card.Level = uint32(cardData.Level)
				card.RScale = uint32(cardData.RScale)
				card.LinkMarker = cardData.LinkMarker
				card.Race = cardData.Race
				card.Attribute = cardData.Attribute
			} else {
				return 0
			}
			return uint(cardId)
		}),
	)
	if err != nil {
		panic(err)
	}

	duel := ocgcore.NewDuel(170000)

	duel.SetErrorHandler(func(message string) {
		slog.Error(message)
	})
	duel.InitPlayers(8000, 5, 1)
	var (
		mainCards = []uint32{14124483, 9411399, 9411399, 18094166, 18094166, 18094166, 40044918, 40044918, 59392529, 50720316, 50720316, 27780618, 27780618, 16605586, 16605586, 22865492, 22865492, 23434538, 23434538, 14558127, 14558127,
			13650422, 83965310, 81439173, 8949584, 8949584, 32807846, 52947044, 45906428, 24094653, 21143940, 21143940, 21143940, 48130397, 24224830, 24224830, 12071500, 24299458, 24299458, 10045474}
		exidCards = []uint32{73580471, 79606837, 79606837, 79606837, 21521304, 27552504, 1174075, 1174075, 1174075, 73898890, 73898890, 72336818, 41999284, 94259633, 94259633}
	)
	for i := len(mainCards) - 1; i >= 0; i-- {
		duel.AddCard(mainCards[i], 0, LOCATION_DECK)
	}
	for i := len(exidCards) - 1; i >= 0; i-- {
		duel.AddCard(exidCards[i], 0, LOCATION_EXTRA)
	}
	for i := len(mainCards) - 1; i >= 0; i-- {
		duel.AddCard(mainCards[i], 1, LOCATION_DECK)
	}
	for i := len(exidCards) - 1; i >= 0; i-- {
		duel.AddCard(exidCards[i], 1, LOCATION_EXTRA)
	}
	qbuf := make([]byte, ocgcore.SIZE_QUERY_BUFFER)
	data := duel.QueryFieldCard(uint8(0), ocgcore.LOCATION_EXTRA, 0xe81fff, qbuf, true)
	fmt.Println(data)
	data = duel.QueryFieldCard(uint8(1), ocgcore.LOCATION_EXTRA, 0xe81fff, qbuf, true)
	fmt.Println(data)
	duel.Start(5)
}
