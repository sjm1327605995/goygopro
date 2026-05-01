package network

const (
	NETWORK_SERVER_ID = 0x7428
	NETWORK_CLIENT_ID = 0xdef6
)

const (
	STOC_GAME_MSG           = 0x1  // byte array
	STOC_ERROR_MSG          = 0x2  // STOC_ErrorMsg
	STOC_SELECT_HAND        = 0x3  // no data
	STOC_SELECT_TP          = 0x4  // no data
	STOC_HAND_RESULT        = 0x5  // STOC_HandResult
	STOC_TP_RESULT          = 0x6  // reserved
	STOC_CHANGE_SIDE        = 0x7  // no data
	STOC_WAITING_SIDE       = 0x8  // no data
	STOC_DECK_COUNT         = 0x9  // int16_t[6]
	STOC_CREATE_GAME        = 0x11 // reserved
	STOC_JOIN_GAME          = 0x12 // STOC_JoinGame
	STOC_TYPE_CHANGE        = 0x13 // STOC_TypeChange
	STOC_LEAVE_GAME         = 0x14 // reserved
	STOC_DUEL_START         = 0x15 // no data
	STOC_DUEL_END           = 0x16 // no data
	STOC_REPLAY             = 0x17 // ReplayHeader + byte array
	STOC_TIME_LIMIT         = 0x18 // STOC_TimeLimit
	STOC_CHAT               = 0x19 // uint16_t + uint16_t array
	STOC_HS_PLAYER_ENTER    = 0x20 // STOC_HS_PlayerEnter
	STOC_HS_PLAYER_CHANGE   = 0x21 // STOC_HS_PlayerChange
	STOC_HS_WATCH_CHANGE    = 0x22 // STOC_HS_WatchChange
	STOC_TEAMMATE_SURRENDER = 0x23 // no data
	STOC_FIELD_FINISH       = 0x30
)
const (
	ERRMSG_JOINERROR = 0x1
	ERRMSG_DECKERROR = 0x2
	ERRMSG_SIDEERROR = 0x3
	ERRMSG_VERERROR  = 0x4
)
const (
	NETPLAYER_TYPE_PLAYER1  = 0
	NETPLAYER_TYPE_PLAYER2  = 1
	NETPLAYER_TYPE_PLAYER3  = 2
	NETPLAYER_TYPE_PLAYER4  = 3
	NETPLAYER_TYPE_PLAYER5  = 4
	NETPLAYER_TYPE_PLAYER6  = 5
	NETPLAYER_TYPE_OBSERVER = 7
)
const (
	PLAYERCHANGE_OBSERVE  = 0x8
	PLAYERCHANGE_READY    = 0x9
	PLAYERCHANGE_NOTREADY = 0xa
	PLAYERCHANGE_LEAVE    = 0xb
)
const (
	DUEL_STAGE_BEGIN   = 0
	DUEL_STAGE_FINGER  = 1
	DUEL_STAGE_FIRSTGO = 2
	DUEL_STAGE_DUELING = 3
	DUEL_STAGE_SIDING  = 4
	DUEL_STAGE_END     = 5
)
const (
	DECKERROR_LFLIST      uint32 = 0x1
	DECKERROR_OCGONLY     uint32 = 0x2
	DECKERROR_TCGONLY     uint32 = 0x3
	DECKERROR_UNKNOWNCARD uint32 = 0x4
	DECKERROR_CARDCOUNT   uint32 = 0x5
	DECKERROR_MAINCOUNT   uint32 = 0x6
	DECKERROR_EXTRACOUNT  uint32 = 0x7
	DECKERROR_SIDECOUNT   uint32 = 0x8
	DECKERROR_NOTAVAIL    uint32 = 0x9
)

const (
	CTOS_RESPONSE      = 0x1  // byte array
	CTOS_UPDATE_DECK   = 0x2  // CTOS_DeckData
	CTOS_HAND_RESULT   = 0x3  // CTOS_HandResult
	CTOS_TP_RESULT     = 0x4  // CTOS_TPResult
	CTOS_PLAYER_INFO   = 0x10 // CTOS_PlayerInfo
	CTOS_CREATE_GAME   = 0x11 // CTOS_CreateGame
	CTOS_JOIN_GAME     = 0x12 // CTOS_JoinGame
	CTOS_LEAVE_GAME    = 0x13 // no data
	CTOS_SURRENDER     = 0x14 // no data
	CTOS_TIME_CONFIRM  = 0x15 // no data
	CTOS_CHAT          = 0x16 // uint16_t array
	CTOS_EXTERNAL_ADDRESS = 0x17 // CTOS_ExternalAddress
	CTOS_HS_TODUELIST  = 0x20 // no data
	CTOS_HS_TOOBSERVER = 0x21 // no data
	CTOS_HS_READY      = 0x22 // no data
	CTOS_HS_NOTREADY   = 0x23 // no data
	CTOS_HS_KICK       = 0x24 // CTOS_Kick
	CTOS_HS_START      = 0x25 // no data
	CTOS_REQUEST_FIELD = 0x30
)
