package duel

import (
	"bufio"
	"bytes"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"github.com/sjm1327605995/goygopro/protocol/network"
	"os"
	"strconv"
	"strings"
)

const (
	DECK_MAX_SIZE  = 60
	DECK_MIN_SIZE  = 40
	EXTRA_MAX_SIZE = 15
	SIDE_MAX_SIZE  = 15
	PACK_MAX_SIZE  = 1000
)
const (
	UnknownString = "???"
)
const (
	AVAIL_OCG    = 0x1
	AVAIL_TCG    = 0x2
	AVAIL_CUSTOM = 0x4
	AVAIL_SC     = 0x8
	AVAIL_OCGTCG = AVAIL_OCG | AVAIL_TCG
)

type deckManager struct {
	deckBuff    *bytes.Buffer
	LFList      []LFList
	currentDeck *Deck

	_datas map[uint32]*CardDataC
}

var DeckManger = new(deckManager)

type LFList struct {
	Hash     uint32
	ListName string
	Content  map[uint32]int
}

func (d *deckManager) LoadLFListSingle(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentList *LFList

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		switch line[0] {
		case '#':
			continue
		case '!':
			listName := line[1:]
			newList := LFList{
				Hash:     0x7dfcee6a,
				ListName: listName,
				Content:  make(map[uint32]int),
			}
			d.LFList = append(d.LFList, newList)
			currentList = &d.LFList[len(d.LFList)-1]
		default:
			if currentList == nil {
				continue
			}

			fields := strings.Fields(line)
			if len(fields) != 2 {
				continue
			}

			codeStr := fields[0]
			countStr := fields[1]

			code, err := strconv.ParseUint(codeStr, 10, 32)
			if err != nil {
				continue
			}

			count, err := strconv.Atoi(countStr)
			if err != nil || count < 0 || count > 2 {
				continue
			}

			code32 := uint32(code)
			currentList.Content[code32] = count
			currentList.Hash = currentList.Hash ^ ((code32 << 18) | (code32 >> 14)) ^ ((code32 << (27 + uint32(count))) | (code32 >> (5 - uint32(count))))
		}
	}
}
func (d *deckManager) LoadLFList() {
	d.LoadLFListSingle("expansions/lflist.conf")
	d.LoadLFListSingle("lflist.conf")

	nolimit := LFList{
		ListName: "N/A",
		Hash:     0,
		Content:  make(map[uint32]int),
	}
	d.LFList = append(d.LFList, nolimit)
}

func (d *deckManager) GetLFListName(lfhash uint32) string {
	for _, list := range d.LFList {
		if list.Hash == lfhash {
			return list.ListName
		}
	}
	return UnknownString
}

func (d *deckManager) GetLFList(lfhash uint32) *LFList {
	for i := range d.LFList {
		if d.LFList[i].Hash == lfhash {
			return &d.LFList[i]
		}
	}
	return nil
}

func checkAvail(ot, avail uint32) uint32 {
	if (ot & avail) == avail {
		return 0
	}
	if (ot&AVAIL_OCG) != 0 && avail != AVAIL_OCG {
		return network.DECKERROR_OCGONLY
	}
	if (ot&AVAIL_TCG) != 0 && avail != AVAIL_TCG {
		return network.DECKERROR_TCGONLY
	}
	return network.DECKERROR_NOTAVAIL
}
func (d *deckManager) CheckDeck(deck *Deck, lfhash uint32, rule int) uint32 {
	ccount := make(map[uint32]int)

	// 检查卡组大小
	if len(deck.Main) < DECK_MIN_SIZE || len(deck.Main) > DECK_MAX_SIZE {
		return (network.DECKERROR_MAINCOUNT << 28) | uint32(len(deck.Main))
	}
	if len(deck.Extra) > EXTRA_MAX_SIZE {
		return (network.DECKERROR_EXTRACOUNT << 28) | uint32(len(deck.Extra))
	}
	if len(deck.Side) > SIDE_MAX_SIZE {
		return (network.DECKERROR_SIDECOUNT << 28) | uint32(len(deck.Side))
	}

	// 获取限制列表
	lflist := d.GetLFList(lfhash)
	if lflist == nil {
		return 0
	}

	// 根据规则确定可用性
	ruleMap := [6]uint32{AVAIL_OCG, AVAIL_TCG, AVAIL_SC,
		AVAIL_CUSTOM, AVAIL_OCGTCG, 0}
	var avail uint32
	if rule >= 0 && rule < len(ruleMap) {
		avail = ruleMap[rule]
	}

	// 检查主卡组
	for _, card := range deck.Main {
		if err := checkAvail(uint32(card.Ot), avail); err != 0 {
			return (err << 28) | card.Code
		}
		if card.Type&(ocgcore.TYPES_EXTRA_DECK|ocgcore.TYPE_TOKEN) != 0 {
			return network.DECKERROR_MAINCOUNT << 28
		}
		code := card.Code
		if card.Alias != 0 {
			code = uint32(card.Alias)
		}
		ccount[code]++
		if ccount[code] > 3 {
			return (network.DECKERROR_CARDCOUNT << 28) | card.Code
		}
		if limit, ok := lflist.Content[code]; ok && ccount[code] > limit {
			return (network.DECKERROR_LFLIST << 28) | card.Code
		}
	}

	// 检查额外卡组
	for _, card := range deck.Extra {
		if err := checkAvail(uint32(card.Ot), avail); err != 0 {
			return (err << 28) | uint32(card.Code)
		}
		if card.Type&ocgcore.TYPES_EXTRA_DECK == 0 || card.Type&ocgcore.TYPE_TOKEN != 0 {
			return network.DECKERROR_EXTRACOUNT << 28
		}
		code := card.Code
		if card.Alias != 0 {
			code = card.Alias
		}
		ccount[code]++
		if ccount[code] > 3 {
			return (network.DECKERROR_CARDCOUNT << 28) | card.Code
		}
		if limit, ok := lflist.Content[code]; ok && ccount[code] > limit {
			return (network.DECKERROR_LFLIST << 28) | card.Code
		}
	}

	// 检查副卡组
	for _, card := range deck.Side {
		if err := checkAvail(uint32(card.Ot), avail); err != 0 {
			return (err << 28) | uint32(card.Code)
		}
		if card.Type&ocgcore.TYPE_TOKEN != 0 {
			return network.DECKERROR_SIDECOUNT << 28
		}
		code := card.Code
		if card.Alias != 0 {
			code = uint32(card.Alias)
		}
		ccount[code]++
		if ccount[code] > 3 {
			return (network.DECKERROR_CARDCOUNT << 28) | card.Code
		}
		if limit, ok := lflist.Content[uint32(code)]; ok && ccount[code] > limit {
			return (network.DECKERROR_LFLIST << 28) | card.Code
		}
	}

	return 0
}

func (d *deckManager) LoadDeck(deck *Deck, dbuf []uint32, mainc, sidec int32, isPacklist bool) uint32 {

	var errorcode uint32
	var cd *ocgcore.CardData

	for i := int32(0); i < mainc; i++ {
		code := dbuf[i]
		cd = DefaultDataManager.GetData(code)
		if cd == nil {
			errorcode = code
			continue
		}
		if cd.Type&ocgcore.TYPE_TOKEN != 0 {
			errorcode = code
			continue
		}
		if isPacklist {
			deck.Main = append(deck.Main, DefaultDataManager.GetCodePointer(code))
			continue
		}
		if cd.Type&ocgcore.TYPES_EXTRA_DECK != 0 {
			if len(deck.Extra) < EXTRA_MAX_SIZE {
				deck.Extra = append(deck.Extra, DefaultDataManager.GetCodePointer(code))
			}
		} else {
			if len(deck.Main) < DECK_MAX_SIZE {
				deck.Main = append(deck.Main, DefaultDataManager.GetCodePointer(code))
			}
		}
	}

	for i := int32(0); i < sidec; i++ {
		code := dbuf[mainc+i]
		cd = DefaultDataManager.GetData(code)
		if cd == nil {
			errorcode = code
			continue
		}
		if cd.Type&ocgcore.TYPE_TOKEN != 0 {
			errorcode = code
			continue
		}
		if len(deck.Side) < SIDE_MAX_SIZE {
			deck.Side = append(deck.Side, DefaultDataManager.GetCodePointer(code))
		}
	}

	return errorcode
}
func (d *deckManager) LoadSide(deck *Deck, dbuf []uint32, mainc, sidec int32) bool {
	pcount := make(map[uint32]int)
	ncount := make(map[uint32]int)

	// 统计原卡组中各卡牌的数量
	for _, card := range deck.Main {
		pcount[card.Code]++
	}
	for _, card := range deck.Extra {
		pcount[card.Code]++
	}
	for _, card := range deck.Side {
		pcount[card.Code]++
	}

	// 加载新卡组
	ndeck := &Deck{}
	d.LoadDeck(ndeck, dbuf, mainc, sidec, false)

	// 检查卡组大小是否一致
	if len(ndeck.Main) != len(deck.Main) || len(ndeck.Extra) != len(deck.Extra) || len(ndeck.Side) != len(deck.Side) {
		return false
	}

	// 统计新卡组中各卡牌的数量
	for _, card := range ndeck.Main {
		ncount[card.Code]++
	}
	for _, card := range ndeck.Extra {
		ncount[card.Code]++
	}
	for _, card := range ndeck.Side {
		ncount[card.Code]++
	}

	// 比较卡牌数量是否一致
	for code, count := range ncount {
		if count != pcount[code] {
			return false
		}
	}

	// 更新原卡组
	*deck = *ndeck
	return true
}
