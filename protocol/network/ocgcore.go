package network

import "github.com/sjm1327605995/goygopro/ocgcore"

// 引擎 MSG_* 消息号，与 ocgcore/common.go 的常量表同源。上游
// （edo9300 核心）随版本增减消息（MSG_SORT_CHAIN 即近期变体），此处不做
// 逐字拷贝而是别名引用：编译期与引擎表绑定，两边不可能悄悄分叉——此前
// 两份独立拷贝只在线上解析失败时才暴露漂移。
const (
	//Messages
	MSG_RETRY                = ocgcore.MSG_RETRY
	MSG_HINT                 = ocgcore.MSG_HINT
	MSG_WAITING              = ocgcore.MSG_WAITING
	MSG_START                = ocgcore.MSG_START
	MSG_WIN                  = ocgcore.MSG_WIN
	MSG_UPDATE_DATA          = ocgcore.MSG_UPDATE_DATA
	MSG_UPDATE_CARD          = ocgcore.MSG_UPDATE_CARD
	MSG_REQUEST_DECK         = ocgcore.MSG_REQUEST_DECK
	MSG_SELECT_BATTLECMD     = ocgcore.MSG_SELECT_BATTLECMD
	MSG_SELECT_IDLECMD       = ocgcore.MSG_SELECT_IDLECMD
	MSG_SELECT_EFFECTYN      = ocgcore.MSG_SELECT_EFFECTYN
	MSG_SELECT_YESNO         = ocgcore.MSG_SELECT_YESNO
	MSG_SELECT_OPTION        = ocgcore.MSG_SELECT_OPTION
	MSG_SELECT_CARD          = ocgcore.MSG_SELECT_CARD
	MSG_SELECT_CHAIN         = ocgcore.MSG_SELECT_CHAIN
	MSG_SELECT_PLACE         = ocgcore.MSG_SELECT_PLACE
	MSG_SELECT_POSITION      = ocgcore.MSG_SELECT_POSITION
	MSG_SELECT_TRIBUTE       = ocgcore.MSG_SELECT_TRIBUTE
	MSG_SORT_CHAIN           = ocgcore.MSG_SORT_CHAIN
	MSG_SELECT_COUNTER       = ocgcore.MSG_SELECT_COUNTER
	MSG_SELECT_SUM           = ocgcore.MSG_SELECT_SUM
	MSG_SELECT_DISFIELD      = ocgcore.MSG_SELECT_DISFIELD
	MSG_SORT_CARD            = ocgcore.MSG_SORT_CARD
	MSG_SELECT_UNSELECT_CARD = ocgcore.MSG_SELECT_UNSELECT_CARD
	MSG_CONFIRM_DECKTOP      = ocgcore.MSG_CONFIRM_DECKTOP
	MSG_CONFIRM_CARDS        = ocgcore.MSG_CONFIRM_CARDS
	MSG_SHUFFLE_DECK         = ocgcore.MSG_SHUFFLE_DECK
	MSG_SHUFFLE_HAND         = ocgcore.MSG_SHUFFLE_HAND
	MSG_REFRESH_DECK         = ocgcore.MSG_REFRESH_DECK
	MSG_SWAP_GRAVE_DECK      = ocgcore.MSG_SWAP_GRAVE_DECK
	MSG_SHUFFLE_SET_CARD     = ocgcore.MSG_SHUFFLE_SET_CARD
	MSG_REVERSE_DECK         = ocgcore.MSG_REVERSE_DECK
	MSG_DECK_TOP             = ocgcore.MSG_DECK_TOP
	MSG_SHUFFLE_EXTRA        = ocgcore.MSG_SHUFFLE_EXTRA
	MSG_NEW_TURN             = ocgcore.MSG_NEW_TURN
	MSG_NEW_PHASE            = ocgcore.MSG_NEW_PHASE
	MSG_CONFIRM_EXTRATOP     = ocgcore.MSG_CONFIRM_EXTRATOP
	MSG_MOVE                 = ocgcore.MSG_MOVE
	MSG_POS_CHANGE           = ocgcore.MSG_POS_CHANGE
	MSG_SET                  = ocgcore.MSG_SET
	MSG_SWAP                 = ocgcore.MSG_SWAP
	MSG_FIELD_DISABLED       = ocgcore.MSG_FIELD_DISABLED
	MSG_SUMMONING            = ocgcore.MSG_SUMMONING
	MSG_SUMMONED             = ocgcore.MSG_SUMMONED
	MSG_SPSUMMONING          = ocgcore.MSG_SPSUMMONING
	MSG_SPSUMMONED           = ocgcore.MSG_SPSUMMONED
	MSG_FLIPSUMMONING        = ocgcore.MSG_FLIPSUMMONING
	MSG_FLIPSUMMONED         = ocgcore.MSG_FLIPSUMMONED
	MSG_CHAINING             = ocgcore.MSG_CHAINING
	MSG_CHAINED              = ocgcore.MSG_CHAINED
	MSG_CHAIN_SOLVING        = ocgcore.MSG_CHAIN_SOLVING
	MSG_CHAIN_SOLVED         = ocgcore.MSG_CHAIN_SOLVED
	MSG_CHAIN_END            = ocgcore.MSG_CHAIN_END
	MSG_CHAIN_NEGATED        = ocgcore.MSG_CHAIN_NEGATED
	MSG_CHAIN_DISABLED       = ocgcore.MSG_CHAIN_DISABLED
	MSG_CARD_SELECTED        = ocgcore.MSG_CARD_SELECTED
	MSG_RANDOM_SELECTED      = ocgcore.MSG_RANDOM_SELECTED
	MSG_BECOME_TARGET        = ocgcore.MSG_BECOME_TARGET
	MSG_DRAW                 = ocgcore.MSG_DRAW
	MSG_DAMAGE               = ocgcore.MSG_DAMAGE
	MSG_RECOVER              = ocgcore.MSG_RECOVER
	MSG_EQUIP                = ocgcore.MSG_EQUIP
	MSG_LPUPDATE             = ocgcore.MSG_LPUPDATE
	MSG_UNEQUIP              = ocgcore.MSG_UNEQUIP
	MSG_CARD_TARGET          = ocgcore.MSG_CARD_TARGET
	MSG_CANCEL_TARGET        = ocgcore.MSG_CANCEL_TARGET
	MSG_PAY_LPCOST           = ocgcore.MSG_PAY_LPCOST
	MSG_ADD_COUNTER          = ocgcore.MSG_ADD_COUNTER
	MSG_REMOVE_COUNTER       = ocgcore.MSG_REMOVE_COUNTER
	MSG_ATTACK               = ocgcore.MSG_ATTACK
	MSG_BATTLE               = ocgcore.MSG_BATTLE
	MSG_ATTACK_DISABLED      = ocgcore.MSG_ATTACK_DISABLED
	MSG_DAMAGE_STEP_START    = ocgcore.MSG_DAMAGE_STEP_START
	MSG_DAMAGE_STEP_END      = ocgcore.MSG_DAMAGE_STEP_END
	MSG_MISSED_EFFECT        = ocgcore.MSG_MISSED_EFFECT
	MSG_BE_CHAIN_TARGET      = ocgcore.MSG_BE_CHAIN_TARGET
	MSG_CREATE_RELATION      = ocgcore.MSG_CREATE_RELATION
	MSG_RELEASE_RELATION     = ocgcore.MSG_RELEASE_RELATION
	MSG_TOSS_COIN            = ocgcore.MSG_TOSS_COIN
	MSG_TOSS_DICE            = ocgcore.MSG_TOSS_DICE
	MSG_ROCK_PAPER_SCISSORS  = ocgcore.MSG_ROCK_PAPER_SCISSORS
	MSG_HAND_RES             = ocgcore.MSG_HAND_RES
	MSG_ANNOUNCE_RACE        = ocgcore.MSG_ANNOUNCE_RACE
	MSG_ANNOUNCE_ATTRIB      = ocgcore.MSG_ANNOUNCE_ATTRIB
	MSG_ANNOUNCE_CARD        = ocgcore.MSG_ANNOUNCE_CARD
	MSG_ANNOUNCE_NUMBER      = ocgcore.MSG_ANNOUNCE_NUMBER
	MSG_ANNOUNCE_CARD_FILTER = ocgcore.MSG_ANNOUNCE_CARD_FILTER
	MSG_CARD_HINT            = ocgcore.MSG_CARD_HINT
	MSG_TAG_SWAP             = ocgcore.MSG_TAG_SWAP
	MSG_RELOAD_FIELD         = ocgcore.MSG_RELOAD_FIELD // Debug.ReloadFieldEnd()
	MSG_AI_NAME              = ocgcore.MSG_AI_NAME
	MSG_SHOW_HINT            = ocgcore.MSG_SHOW_HINT
	MSG_PLAYER_HINT          = ocgcore.MSG_PLAYER_HINT
	MSG_MATCH_KILL           = ocgcore.MSG_MATCH_KILL
	MSG_CUSTOM_MSG           = ocgcore.MSG_CUSTOM_MSG
	MSG_DUEL_WINNER          = ocgcore.MSG_DUEL_WINNER
)
