package ocgcore

//import (
//	"database/sql"
//)
//
//// Card 表示游戏中的一张卡片
//type Card struct {
//	ID         uint32
//	Ot         int
//	Alias      int
//	Setcode    int64
//	Type       int
//	Level      int
//	LScale     int
//	RScale     int
//	LinkMarker int
//	Attribute  int
//	Race       int
//	Attack     int
//	Defense    int
//	Data       CardData
//}
//
//// Get 根据ID获取卡片
//func Get(id int) *Card {
//	return GetCard(id)
//}
//
//// HasType 检查卡片是否具有指定类型
//func (c *Card) HasType(ct CardType) bool {
//	return (c.Type & int(ct)) != 0
//}
//
//// IsExtraCard 检查卡片是否是额外卡组的卡
//func (c *Card) IsExtraCard() bool {
//	return c.HasType(CardTypeFusion) ||
//		c.HasType(CardTypeSynchro) ||
//		c.HasType(CardTypeXyz) ||
//		c.HasType(CardTypeLink)
//}
//
//// NewCard 从数据库记录创建新卡片
//func NewCard(rows *sql.Rows) (*Card, error) {
//	var id, ot, alias, cardType, levelInfo, race, attribute, attack, defense int
//	var setcode int64
//
//	err := rows.Scan(&id, &ot, &alias, &setcode, &cardType, &levelInfo, &race, &attribute, &attack, &defense)
//	if err != nil {
//		return nil, err
//	}
//
//	card := &Card{
//		ID:        id,
//		Ot:        ot,
//		Alias:     alias,
//		Setcode:   setcode,
//		Type:      cardType,
//		Level:     levelInfo & 0xff,
//		LScale:    (levelInfo >> 24) & 0xff,
//		RScale:    (levelInfo >> 16) & 0xff,
//		Race:      race,
//		Attribute: attribute,
//		Attack:    attack,
//		Defense:   defense,
//	}
//
//	if card.HasType(CardTypeLink) {
//		card.LinkMarker = defense
//		card.Defense = 0
//	}
//
//	card.Data = CardData{
//		Code:       uint32(card.ID),
//		Alias:      uint32(card.Alias),
//		Setcode:    card.Setcode,
//		Type:       uint32(card.Type),
//		Level:      uint32(card.Level),
//		Attribute:  uint32(card.Attribute),
//		Race:       uint32(card.Race),
//		Attack:     int32(card.Attack),
//		Defense:    int32(card.Defense),
//		LScale:     uint32(card.LScale),
//		RScale:     uint32(card.RScale),
//		LinkMarker: uint32(card.LinkMarker),
//	}
//
//	return card, nil
//}
