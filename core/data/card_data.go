package data

import "github.com/sjm1327605995/goygopro/ocgcore"

type CardDataC struct {
	Ot       uint32
	Category int64
	// RuleCode 是原版 rule_code 字段：LoadDB 时把非异画 alias 迁入并清零
	// alias（data_manager.cpp:61-68），卡数/禁限校验按 get_duel_code()
	// 即 RuleCode ?: Alias ?: Code 计数（deck_manager.cpp:109/124/139）。
	RuleCode uint32
	ocgcore.CardData
}

// DuelCode 对应原版 CardDataC::get_duel_code()（data_manager.h:60-62）。
func (card *CardDataC) DuelCode() uint32 {
	if card.RuleCode != 0 {
		return card.RuleCode
	}
	if card.Alias != 0 {
		return card.Alias
	}
	return card.Code
}

type CardString struct {
	Name string
	Text string
	Desc [16]string
}
