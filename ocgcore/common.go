package ocgcore

func checkPlayerId(playerId int32) bool {
	return playerId >= 0 && playerId <= 1
}

// MatchAll 检查x是否包含y的所有位
func MatchAll(x, y uint32) bool {
	return (x & y) == y
}

// MatchAny 检查x和y是否有任何相同的位
func MatchAny(x, y uint32) bool {
	return (x & y) != 0
}

// AddBit 在x中添加y的位，返回新值
func AddBit(x *uint32, y uint32) {
	*x |= y
}

// RemoveBit 从x中移除y的位，返回新值
func RemoveBit(x *uint32, y uint32) {
	*x &^= y // Go的"AND NOT"操作符
}

const (
	LEN_FAIL     = 0
	LEN_EMPTY    = 4
	LEN_HEADER   = 8
	TEMP_CARD_ID = 0
)

const (
	OPERATION_SUCCESS   = 1
	OPERATION_FAIL      = 0
	OPERATION_CANCELED  = -1
	TRUE                = 1
	FALSE               = 0
	SIZE_MESSAGE_BUFFER = 0x2000
	SIZE_RETURN_VALUE   = 512
)
const (
	PROCESSOR_BUFFER_LEN = 0x0fffffff
	PROCESSOR_FLAG       = 0xf0000000
	PROCESSOR_NONE       = 0
	PROCESSOR_WAITING    = 0x10000000
	PROCESSOR_END        = 0x20000000
)
const (
	MASTER_RULE3     = 3 //Master Rule 3 (2014)
	NEW_MASTER_RULE  = 4 //New Master Rule (2017)
	MASTER_RULE_2020 = 5 //Master Rule 2020
	CURRENT_RULE     = 5
)

// Locations
const (
	LOCATION_DECK    uint8 = 0x01
	LOCATION_HAND    uint8 = 0x02
	LOCATION_MZONE   uint8 = 0x04
	LOCATION_SZONE   uint8 = 0x08
	LOCATION_GRAVE   uint8 = 0x10
	LOCATION_REMOVED uint8 = 0x20
	LOCATION_EXTRA   uint8 = 0x40
	LOCATION_OVERLAY uint8 = 0x80
	LOCATION_ONFIELD uint8 = LOCATION_MZONE | LOCATION_SZONE

	LOCATION_FZONE uint32 = 0x100
	LOCATION_PZONE uint32 = 0x200
	//For redirect
	LOCATION_DECKBOT uint32 = 0x10001 //Return to deck bottom
	LOCATION_DECKSHF uint32 = 0x20001 //Return to deck and shuffle
)
const (
	//For Duel.SendtoDeck
	SEQ_DECKTOP     = 0 //Return to deck top
	SEQ_DECKBOTTOM  = 1 //Return to deck bottom
	SEQ_DECKSHUFFLE = 2 //Return to deck and shuffle
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

// Flip effect flags
const NO_FLIP_EFFECT = 0x10000

// Move to field flags
const (
	RETURN_TEMP_REMOVE_TO_FIELD  = 1
	RETURN_TRAP_MONSTER_TO_SZONE = 2
)

// Types
const (
	TYPE_MONSTER     = 0x1       //
	TYPE_SPELL       = 0x2       //
	TYPE_TRAP        = 0x4       //
	TYPE_NORMAL      = 0x10      //
	TYPE_EFFECT      = 0x20      //
	TYPE_FUSION      = 0x40      //
	TYPE_RITUAL      = 0x80      //
	TYPE_TRAPMONSTER = 0x100     //
	TYPE_SPIRIT      = 0x200     //
	TYPE_UNION       = 0x400     //
	TYPE_DUAL        = 0x800     //
	TYPE_TUNER       = 0x1000    //
	TYPE_SYNCHRO     = 0x2000    //
	TYPE_TOKEN       = 0x4000    //
	TYPE_QUICKPLAY   = 0x10000   //
	TYPE_CONTINUOUS  = 0x20000   //
	TYPE_EQUIP       = 0x40000   //
	TYPE_FIELD       = 0x80000   //
	TYPE_COUNTER     = 0x100000  //
	TYPE_FLIP        = 0x200000  //
	TYPE_TOON        = 0x400000  //
	TYPE_XYZ         = 0x800000  //
	TYPE_PENDULUM    = 0x1000000 //
	TYPE_SPSUMMON    = 0x2000000 //
	TYPE_LINK        = 0x4000000 //

	TYPES_EXTRA_DECK = TYPE_FUSION | TYPE_SYNCHRO | TYPE_XYZ | TYPE_LINK
)

// Attributes
const (
	ATTRIBUTES_COUNT = 7
	ATTRIBUTE_ALL    = 0x7f //
	ATTRIBUTE_EARTH  = 0x01 //
	ATTRIBUTE_WATER  = 0x02 //
	ATTRIBUTE_FIRE   = 0x04 //
	ATTRIBUTE_WIND   = 0x08 //
	ATTRIBUTE_LIGHT  = 0x10 //
	ATTRIBUTE_DARK   = 0x20 //
	ATTRIBUTE_DEVINE = 0x40 //
)

// Races
const (
	RACES_COUNT       = 26
	RACE_ALL          = 0x3ffffff
	RACE_WARRIOR      = 0x1       //
	RACE_SPELLCASTER  = 0x2       //
	RACE_FAIRY        = 0x4       //
	RACE_FIEND        = 0x8       //
	RACE_ZOMBIE       = 0x10      //
	RACE_MACHINE      = 0x20      //
	RACE_AQUA         = 0x40      //
	RACE_PYRO         = 0x80      //
	RACE_ROCK         = 0x100     //
	RACE_WINDBEAST    = 0x200     //
	RACE_PLANT        = 0x400     //
	RACE_INSECT       = 0x800     //
	RACE_THUNDER      = 0x1000    //
	RACE_DRAGON       = 0x2000    //
	RACE_BEAST        = 0x4000    //
	RACE_BEASTWARRIOR = 0x8000    //
	RACE_DINOSAUR     = 0x10000   //
	RACE_FISH         = 0x20000   //
	RACE_SEASERPENT   = 0x40000   //
	RACE_REPTILE      = 0x80000   //
	RACE_PSYCHO       = 0x100000  //
	RACE_DEVINE       = 0x200000  //
	RACE_CREATORGOD   = 0x400000  //
	RACE_WYRM         = 0x800000  //
	RACE_CYBERSE      = 0x1000000 //
	RACE_ILLUSION     = 0x2000000 //
)

// Reason
const (
	REASON_DESTROY      = 0x1        //
	REASON_RELEASE      = 0x2        //
	REASON_TEMPORARY    = 0x4        //
	REASON_MATERIAL     = 0x8        //
	REASON_SUMMON       = 0x10       //
	REASON_BATTLE       = 0x20       //
	REASON_EFFECT       = 0x40       //
	REASON_COST         = 0x80       //
	REASON_ADJUST       = 0x100      //
	REASON_LOST_TARGET  = 0x200      //
	REASON_RULE         = 0x400      //
	REASON_SPSUMMON     = 0x800      //
	REASON_DISSUMMON    = 0x1000     //
	REASON_FLIP         = 0x2000     //
	REASON_DISCARD      = 0x4000     //
	REASON_RDAMAGE      = 0x8000     //
	REASON_RRECOVER     = 0x10000    //
	REASON_RETURN       = 0x20000    //
	REASON_FUSION       = 0x40000    //
	REASON_SYNCHRO      = 0x80000    //
	REASON_RITUAL       = 0x100000   //
	REASON_XYZ          = 0x200000   //
	REASON_REPLACE      = 0x1000000  //
	REASON_DRAW         = 0x2000000  //
	REASON_REDIRECT     = 0x4000000  //
	REASON_REVEAL       = 0x8000000  //
	REASON_LINK         = 0x10000000 //
	REASON_LOST_OVERLAY = 0x20000000 //
	REASON_MAINTENANCE  = 0x40000000 //
	REASON_ACTION       = 0x80000000 //

	REASONS_PROCEDURE = REASON_SYNCHRO | REASON_XYZ | REASON_LINK
)

// Status
const (
	STATUS_DISABLED                = 0x0001
	STATUS_TO_ENABLE               = 0x0002
	STATUS_TO_DISABLE              = 0x0004
	STATUS_PROC_COMPLETE           = 0x0008
	STATUS_SET_TURN                = 0x0010
	STATUS_NO_LEVEL                = 0x0020
	STATUS_BATTLE_RESULT           = 0x0040
	STATUS_SPSUMMON_STEP           = 0x0080
	STATUS_CANNOT_CHANGE_FORM      = 0x0100
	STATUS_SUMMONING               = 0x0200
	STATUS_EFFECT_ENABLED          = 0x0400
	STATUS_SUMMON_TURN             = 0x0800
	STATUS_DESTROY_CONFIRMED       = 0x1000
	STATUS_LEAVE_CONFIRMED         = 0x2000
	STATUS_BATTLE_DESTROYED        = 0x4000
	STATUS_COPYING_EFFECT          = 0x8000
	STATUS_CHAINING                = 0x10000
	STATUS_SUMMON_DISABLED         = 0x20000
	STATUS_ACTIVATE_DISABLED       = 0x40000
	STATUS_EFFECT_REPLACED         = 0x80000
	STATUS_FLIP_SUMMONING          = 0x100000
	STATUS_ATTACK_CANCELED         = 0x200000
	STATUS_INITIALIZING            = 0x400000
	STATUS_TO_HAND_WITHOUT_CONFIRM = 0x800000
	STATUS_JUST_POS                = 0x1000000
	STATUS_CONTINUOUS_POS          = 0x2000000
	STATUS_FORBIDDEN               = 0x4000000
	STATUS_ACT_FROM_HAND           = 0x8000000
	STATUS_OPPO_BATTLE             = 0x10000000
	STATUS_FLIP_SUMMON_TURN        = 0x20000000
	STATUS_SPSUMMON_TURN           = 0x40000000
	STATUS_FLIP_SUMMON_DISABLED    = 0x80000000
)

// Query list
const (
	QUERY_CODE         = 0x1
	QUERY_POSITION     = 0x2
	QUERY_ALIAS        = 0x4
	QUERY_TYPE         = 0x8
	QUERY_LEVEL        = 0x10
	QUERY_RANK         = 0x20
	QUERY_ATTRIBUTE    = 0x40
	QUERY_RACE         = 0x80
	QUERY_ATTACK       = 0x100
	QUERY_DEFENSE      = 0x200
	QUERY_BASE_ATTACK  = 0x400
	QUERY_BASE_DEFENSE = 0x800
	QUERY_REASON       = 0x1000
	QUERY_REASON_CARD  = 0x2000
	QUERY_EQUIP_CARD   = 0x4000
	QUERY_TARGET_CARD  = 0x8000
	QUERY_OVERLAY_CARD = 0x10000
	QUERY_COUNTERS     = 0x20000
	QUERY_OWNER        = 0x40000
	QUERY_STATUS       = 0x80000
	QUERY_LSCALE       = 0x200000
	QUERY_RSCALE       = 0x400000
	QUERY_LINK         = 0x800000
)

// Link markers
const (
	LINK_MARKER_BOTTOM_LEFT  = 0x001
	LINK_MARKER_BOTTOM       = 0x002
	LINK_MARKER_BOTTOM_RIGHT = 0x004
	LINK_MARKER_LEFT         = 0x008
	LINK_MARKER_RIGHT        = 0x020
	LINK_MARKER_TOP_LEFT     = 0x040
	LINK_MARKER_TOP          = 0x080
	LINK_MARKER_TOP_RIGHT    = 0x100
)

// Messages
const (
	MSG_RETRY                = 1
	MSG_HINT                 = 2
	MSG_WAITING              = 3
	MSG_START                = 4
	MSG_WIN                  = 5
	MSG_UPDATE_DATA          = 6
	MSG_UPDATE_CARD          = 7
	MSG_REQUEST_DECK         = 8
	MSG_SELECT_BATTLECMD     = 10
	MSG_SELECT_IDLECMD       = 11
	MSG_SELECT_EFFECTYN      = 12
	MSG_SELECT_YESNO         = 13
	MSG_SELECT_OPTION        = 14
	MSG_SELECT_CARD          = 15
	MSG_SELECT_CHAIN         = 16
	MSG_SELECT_PLACE         = 18
	MSG_SELECT_POSITION      = 19
	MSG_SELECT_TRIBUTE       = 20
	MSG_SELECT_COUNTER       = 22
	MSG_SELECT_SUM           = 23
	MSG_SELECT_DISFIELD      = 24
	MSG_SORT_CARD            = 25
	MSG_SELECT_UNSELECT_CARD = 26
	MSG_CONFIRM_DECKTOP      = 30
	MSG_CONFIRM_CARDS        = 31
	MSG_SHUFFLE_DECK         = 32
	MSG_SHUFFLE_HAND         = 33
	MSG_REFRESH_DECK         = 34
	MSG_SWAP_GRAVE_DECK      = 35
	MSG_SHUFFLE_SET_CARD     = 36
	MSG_REVERSE_DECK         = 37
	MSG_DECK_TOP             = 38
	MSG_SHUFFLE_EXTRA        = 39
	MSG_NEW_TURN             = 40
	MSG_NEW_PHASE            = 41
	MSG_CONFIRM_EXTRATOP     = 42
	MSG_MOVE                 = 50
	MSG_POS_CHANGE           = 53
	MSG_SET                  = 54
	MSG_SWAP                 = 55
	MSG_FIELD_DISABLED       = 56
	MSG_SUMMONING            = 60
	MSG_SUMMONED             = 61
	MSG_SPSUMMONING          = 62
	MSG_SPSUMMONED           = 63
	MSG_FLIPSUMMONING        = 64
	MSG_FLIPSUMMONED         = 65
	MSG_CHAINING             = 70
	MSG_CHAINED              = 71
	MSG_CHAIN_SOLVING        = 72
	MSG_CHAIN_SOLVED         = 73
	MSG_CHAIN_END            = 74
	MSG_CHAIN_NEGATED        = 75
	MSG_CHAIN_DISABLED       = 76
	MSG_CARD_SELECTED        = 80
	MSG_RANDOM_SELECTED      = 81
	MSG_BECOME_TARGET        = 83
	MSG_DRAW                 = 90
	MSG_DAMAGE               = 91
	MSG_RECOVER              = 92
	MSG_EQUIP                = 93
	MSG_LPUPDATE             = 94
	MSG_UNEQUIP              = 95
	MSG_CARD_TARGET          = 96
	MSG_CANCEL_TARGET        = 97
	MSG_PAY_LPCOST           = 100
	MSG_ADD_COUNTER          = 101
	MSG_REMOVE_COUNTER       = 102
	MSG_ATTACK               = 110
	MSG_BATTLE               = 111
	MSG_ATTACK_DISABLED      = 112
	MSG_DAMAGE_STEP_START    = 113
	MSG_DAMAGE_STEP_END      = 114
	MSG_MISSED_EFFECT        = 120
	MSG_BE_CHAIN_TARGET      = 121
	MSG_CREATE_RELATION      = 122
	MSG_RELEASE_RELATION     = 123
	MSG_TOSS_COIN            = 130
	MSG_TOSS_DICE            = 131
	MSG_ROCK_PAPER_SCISSORS  = 132
	MSG_HAND_RES             = 133
	MSG_ANNOUNCE_RACE        = 140
	MSG_ANNOUNCE_ATTRIB      = 141
	MSG_ANNOUNCE_CARD        = 142
	MSG_ANNOUNCE_NUMBER      = 143
	MSG_CARD_HINT            = 160
	MSG_TAG_SWAP             = 161
	MSG_RELOAD_FIELD         = 162 // Debug.ReloadFieldEnd()
	MSG_AI_NAME              = 163
	MSG_SHOW_HINT            = 164
	MSG_PLAYER_HINT          = 165
	MSG_MATCH_KILL           = 170
	MSG_CUSTOM_MSG           = 180
)

// Hints
const (
	HINT_EVENT      = 1
	HINT_MESSAGE    = 2
	HINT_SELECTMSG  = 3
	HINT_OPSELECTED = 4
	HINT_EFFECT     = 5
	HINT_RACE       = 6
	HINT_ATTRIB     = 7
	HINT_CODE       = 8
	HINT_NUMBER     = 9
	HINT_CARD       = 10
	HINT_ZONE       = 11
)

const (
	CHINT_TURN        = 1
	CHINT_CARD        = 2
	CHINT_RACE        = 3
	CHINT_ATTRIBUTE   = 4
	CHINT_NUMBER      = 5
	CHINT_DESC_ADD    = 6
	CHINT_DESC_REMOVE = 7
)

const (
	PHINT_DESC_ADD    = 6
	PHINT_DESC_REMOVE = 7
)

const (
	EDESC_OPERATION = 1
	EDESC_RESET     = 2
)

const (
	OPCODE_ADD         = 0x40000000
	OPCODE_SUB         = 0x40000001
	OPCODE_MUL         = 0x40000002
	OPCODE_DIV         = 0x40000003
	OPCODE_AND         = 0x40000004
	OPCODE_OR          = 0x40000005
	OPCODE_NEG         = 0x40000006
	OPCODE_NOT         = 0x40000007
	OPCODE_ISCODE      = 0x40000100
	OPCODE_ISSETCARD   = 0x40000101
	OPCODE_ISTYPE      = 0x40000102
	OPCODE_ISRACE      = 0x40000103
	OPCODE_ISATTRIBUTE = 0x40000104
)

// Player
const (
	PLAYER_NONE    = 2 //
	PLAYER_ALL     = 3 //
	PLAYER_SELFDES = 5 //
)

// Phase
const (
	PHASE_DRAW         = 0x01
	PHASE_STANDBY      = 0x02
	PHASE_MAIN1        = 0x04
	PHASE_BATTLE_START = 0x08
	PHASE_BATTLE_STEP  = 0x10
	PHASE_DAMAGE       = 0x20
	PHASE_DAMAGE_CAL   = 0x40
	PHASE_BATTLE       = 0x80
	PHASE_MAIN2        = 0x100
	PHASE_END          = 0x200
)

// Options
const (
	DUEL_TEST_MODE         = 0x01
	DUEL_ATTACK_FIRST_TURN = 0x02
	DUEL_OLD_REPLAY        = 0x04
	DUEL_OBSOLETE_RULING   = 0x08
	DUEL_PSEUDO_SHUFFLE    = 0x10
	DUEL_TAG_MODE          = 0x20
	DUEL_SIMPLE_AI         = 0x40
	DUEL_RETURN_DECK_TOP   = 0x80
	DUEL_REVEAL_DECK_SEQ   = 0x100
)

// Activity
const (
	ACTIVITY_SUMMON       = 1
	ACTIVITY_NORMALSUMMON = 2
	ACTIVITY_SPSUMMON     = 3
	ACTIVITY_FLIPSUMMON   = 4
	ACTIVITY_ATTACK       = 5
	ACTIVITY_BATTLE_PHASE = 6
	ACTIVITY_CHAIN        = 7
)
