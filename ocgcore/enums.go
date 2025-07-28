package ocgcore

type GameMsg byte

const (
	GameMsgRetry              GameMsg = 1
	GameMsgHint                       = 2
	GameMsgWaiting                    = 3
	GameMsgStart                      = 4
	GameMsgWin                        = 5
	GameMsgUpdateData                 = 6
	GameMsgUpdateCard                 = 7
	GameMsgRequestDeck                = 8
	GameMsgSelectBattleCmd            = 10
	GameMsgSelectIdleCmd              = 11
	GameMsgSelectEffectYn             = 12
	GameMsgSelectYesNo                = 13
	GameMsgSelectOption               = 14
	GameMsgSelectCard                 = 15
	GameMsgSelectChain                = 16
	GameMsgSelectPlace                = 18
	GameMsgSelectPosition             = 19
	GameMsgSelectTribute              = 20
	GameMsgSortChain                  = 21
	GameMsgSelectCounter              = 22
	GameMsgSelectSum                  = 23
	GameMsgSelectDisfield             = 24
	GameMsgSortCard                   = 25
	GameMsgSelectUnselect             = 26
	GameMsgConfirmDecktop             = 30
	GameMsgConfirmCards               = 31
	GameMsgShuffleDeck                = 32
	GameMsgShuffleHand                = 33
	GameMsgRefreshDeck                = 34
	GameMsgSwapGraveDeck              = 35
	GameMsgShuffleSetCard             = 36
	GameMsgReverseDeck                = 37
	GameMsgDeckTop                    = 38
	GameMsgShuffleExtra               = 39
	GameMsgNewTurn                    = 40
	GameMsgNewPhase                   = 41
	GameMsgConfirmExtratop            = 42
	GameMsgMove                       = 50
	GameMsgPosChange                  = 53
	GameMsgSet                        = 54
	GameMsgSwap                       = 55
	GameMsgFieldDisabled              = 56
	GameMsgSummoning                  = 60
	GameMsgSummoned                   = 61
	GameMsgSpSummoning                = 62
	GameMsgSpSummoned                 = 63
	GameMsgFlipSummoning              = 64
	GameMsgFlipSummoned               = 65
	GameMsgChaining                   = 70
	GameMsgChained                    = 71
	GameMsgChainSolving               = 72
	GameMsgChainSolved                = 73
	GameMsgChainEnd                   = 74
	GameMsgChainNegated               = 75
	GameMsgChainDisabled              = 76
	GameMsgCardSelected               = 80
	GameMsgRandomSelected             = 81
	GameMsgBecomeTarget               = 83
	GameMsgDraw                       = 90
	GameMsgDamage                     = 91
	GameMsgRecover                    = 92
	GameMsgEquip                      = 93
	GameMsgLpUpdate                   = 94
	GameMsgUnequip                    = 95
	GameMsgCardTarget                 = 96
	GameMsgCancelTarget               = 97
	GameMsgPayLpCost                  = 100
	GameMsgAddCounter                 = 101
	GameMsgRemoveCounter              = 102
	GameMsgAttack                     = 110
	GameMsgBattle                     = 111
	GameMsgAttackDisabled             = 112
	GameMsgDamageStepStart            = 113
	GameMsgDamageStepEnd              = 114
	GameMsgMissedEffect               = 120
	GameMsgBeChainTarget              = 121
	GameMsgCreateRelation             = 122
	GameMsgReleaseRelation            = 123
	GameMsgTossCoin                   = 130
	GameMsgTossDice                   = 131
	GameMsgRockPaperScissors          = 132
	GameMsgHandResult                 = 133
	GameMsgAnnounceRace               = 140
	GameMsgAnnounceAttrib             = 141
	GameMsgAnnounceCard               = 142
	GameMsgAnnounceNumber             = 143
	GameMsgAnnounceCardFilter         = 144
	GameMsgCardHint                   = 160
	GameMsgTagSwap                    = 161
	GameMsgReloadField                = 162
	GameMsgAiName                     = 163
	GameMsgShowHint                   = 164
	GameMsgPlayerHint                 = 165
	GameMsgMatchKill                  = 170
	GameMsgCustomMsg                  = 180
	GameMsgDuelWinner                 = 200
)

// CardType 定义了卡片的类型
type CardType uint32

const (
	CardTypeMonster    CardType = 0x1
	CardTypeSpell      CardType = 0x2
	CardTypeTrap       CardType = 0x4
	CardTypeNormal     CardType = 0x10
	CardTypeEffect     CardType = 0x20
	CardTypeFusion     CardType = 0x40
	CardTypeRitual     CardType = 0x80
	CardTypeTrapMon    CardType = 0x100
	CardTypeSpirit     CardType = 0x200
	CardTypeUnion      CardType = 0x400
	CardTypeGemini     CardType = 0x800
	CardTypeTuner      CardType = 0x1000
	CardTypeSynchro    CardType = 0x2000
	CardTypeToken      CardType = 0x4000
	CardTypeQuickPlay  CardType = 0x10000
	CardTypeContinuous CardType = 0x20000
	CardTypeEquip      CardType = 0x40000
	CardTypeField      CardType = 0x80000
	CardTypeCounter    CardType = 0x100000
	CardTypeFlip       CardType = 0x200000
	CardTypeToon       CardType = 0x400000
	CardTypeXyz        CardType = 0x800000
	CardTypePendulum   CardType = 0x1000000
	CardTypeLink       CardType = 0x2000000
)

// CardLocation 定义了卡片的位置

const (
	CardLocationDeck        = 0x01
	CardLocationHand        = 0x02
	CardLocationMonsterZone = 0x04
	CardLocationSpellZone   = 0x08
	CardLocationGrave       = 0x10
	CardLocationRemoved     = 0x20
	CardLocationExtra       = 0x40
	CardLocationOverlay     = 0x80
	CardLocationOnField     = 0x0C
)

type Query uint8

const (
	QueryCode        Query = 0x01
	QueryPosition          = 0x02
	QueryAlias             = 0x04
	QueryType              = 0x08
	QueryLevel             = 0x10
	QueryRank              = 0x20
	QueryAttribute         = 0x40
	QueryRace              = 0x80
	QueryAttack            = 0x100
	QueryDefence           = 0x200
	QueryBaseAttack        = 0x400
	QueryBaseDefence       = 0x800
	QueryReason            = 0x1000
	QueryReasonCard        = 0x2000
	QueryEquipCard         = 0x4000
	QueryTargetCard        = 0x8000
	QueryOverlayCard       = 0x10000
	QueryCounters          = 0x20000
	QueryOwner             = 0x40000
	QueryStatus            = 0x80000
	QueryLScale            = 0x200000
	QueryRScale            = 0x400000
	QueryLink              = 0x800000
)

type CardPosition uint32

const (
	CardPositionFaceUpAttack    CardPosition = 0x1
	CardPositionFaceDownAttack  CardPosition = 0x2
	CardPositionFaceUpDefence   CardPosition = 0x4
	CardPositionFaceDownDefence CardPosition = 0x8
	CardPositionFaceUp          CardPosition = 0x5
	CardPositionFaceDown        CardPosition = 0xA
	CardPositionAttack          CardPosition = 0x3
	CardPositionDefence         CardPosition = 0xC
)
