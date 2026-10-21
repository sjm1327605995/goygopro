package data

import (
	"github.com/jmoiron/sqlx"
	"github.com/sjm1327605995/goygopro/ocgcore"
	_ "modernc.org/sqlite"
)

const MAX_STRING_ID = 0x7ff
const MIN_CARD_ID = uint32((MAX_STRING_ID + 1) >> 4)
const MAX_CARD_ID = uint32(0x0fffffff)

var (
	stringPointer = make(map[uint32]*CardString, 32768)
	// extraSetCode 对应原版 data_manager.cpp:24-27 的硬编码补丁：两张卡的
	// 系列码无条件覆盖 DB 值。
	extraSetCode = map[uint32][]uint16{
		8512558:  {0x8f, 0x54, 0x59, 0x82, 0x13a},
		55088578: {0x8f, 0x54, 0x59, 0x82, 0x13a},
	}
	datas = make(map[uint32]*CardDataC, 32768)
)
var DefaultDataManager = new(DataManager)

type DataManager struct {
}

func (dm *DataManager) LoadDB(file string) error {
	db, err := sqlx.Open("sqlite", file)
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := db.Query("select datas.id, ot, alias ,setcode ,type ,atk, def, level, race ,attribute, category ,name, desc," +
		"str1, str2, str3 ,str4 ,str5, str6, str7, str8, str9, str10, str11, str12 ,str13, str14,str15 ,str16 from datas,texts where datas.id=texts.id")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			card     CardDataC
			cardStr  CardString
			setCode  uint64
			level    uint32
			scanList = []any{&card.Code, &card.Ot, &card.Alias, &setCode, &card.Type, &card.Attack, &card.Defense, &level, &card.Race, &card.Attribute,
				&card.Category, &cardStr.Name, &cardStr.Text}
		)
		for i := range cardStr.Desc {
			scanList = append(scanList, &cardStr.Desc[i])
		}
		err = rows.Scan(scanList...)
		if err != nil {
			return err
		}
		card.SetSetCode(setCode)
		// rule_code 推导（data_manager.cpp:61-68）：5405695 特例与其余卡的
		// 非异画 alias 一律迁入 rule_code 并清零 alias。
		if card.Code == ocgcore.CARD_BLACK_LUSTER_SOLDIER2 {
			card.RuleCode = card.Alias
			card.Alias = 0
		} else if card.Alias != 0 && card.Type&ocgcore.TYPE_TOKEN == 0 && !isAlternativeCode(card.Code, card.Alias) {
			card.RuleCode = card.Alias
			card.Alias = 0
		}
		if card.Type&ocgcore.TYPE_LINK != 0 {
			card.LinkMarker = uint32(card.Defense)
			card.Defense = 0
		} else {
			card.LinkMarker = 0
		}
		card.Level = level & 0xff
		card.LScale = (level >> 24) & 0xff
		card.RScale = (level >> 16) & 0xff
		datas[card.Code] = &card
		stringPointer[card.Code] = &cardStr
	}
	// 全表后处理（data_manager.cpp:86-104）：
	// 1) 仍带 alias 的卡（异画等）继承 alias 目标的 rule_code；
	// 2) extra_setcode 无条件覆盖对应卡的系列码。
	for _, card := range datas {
		if card.RuleCode != 0 || card.Alias == 0 || card.Type&ocgcore.TYPE_TOKEN != 0 {
			continue
		}
		if target, ok := datas[card.Alias]; ok {
			card.RuleCode = target.RuleCode
		}
	}
	for code, list := range extraSetCode {
		card, ok := datas[code]
		if !ok || len(list) == 0 || len(list) > ocgcore.SIZE_SETCODE {
			continue
		}
		copy(card.Setcode[:], list)
	}
	// 迭代中途出错（I/O/解码）时拒绝半个卡库：静默加载残缺数据会让决斗
	// 按不完整的数据跑，比启动失败更难排查。
	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}

// isAlternativeCode 对应原版 is_alternative（data_manager.cpp:19-21）：
// 互为异画的卡号（ artwork 版本偏移内）不做 rule_code 迁移。
func isAlternativeCode(code, alias uint32) bool {
	offset := uint32(ocgcore.CARD_ARTWORK_VERSIONS_OFFSET)
	return alias != 0 && alias < code+offset && code < alias+offset
}
func (dm *DataManager) GetCodePointer(code uint32) *CardDataC {
	return datas[code]
}

const (
	//Types。逐字别名 ocgcore/common.go 的常量表（不在此处拷贝），与引擎侧
	//不可能悄悄分叉。
	TYPE_MONSTER     = ocgcore.TYPE_MONSTER     //
	TYPE_SPELL       = ocgcore.TYPE_SPELL       //
	TYPE_TRAP        = ocgcore.TYPE_TRAP        //
	TYPE_NORMAL      = ocgcore.TYPE_NORMAL      //
	TYPE_EFFECT      = ocgcore.TYPE_EFFECT      //
	TYPE_FUSION      = ocgcore.TYPE_FUSION      //
	TYPE_RITUAL      = ocgcore.TYPE_RITUAL      //
	TYPE_TRAPMONSTER = ocgcore.TYPE_TRAPMONSTER //
	TYPE_SPIRIT      = ocgcore.TYPE_SPIRIT      //
	TYPE_UNION       = ocgcore.TYPE_UNION       //
	TYPE_DUAL        = ocgcore.TYPE_DUAL        //
	TYPE_TUNER       = ocgcore.TYPE_TUNER       //
	TYPE_SYNCHRO     = ocgcore.TYPE_SYNCHRO     //
	TYPE_TOKEN       = ocgcore.TYPE_TOKEN       //
	TYPE_QUICKPLAY   = ocgcore.TYPE_QUICKPLAY   //
	TYPE_CONTINUOUS  = ocgcore.TYPE_CONTINUOUS  //
	TYPE_EQUIP       = ocgcore.TYPE_EQUIP       //
	TYPE_FIELD       = ocgcore.TYPE_FIELD       //
	TYPE_COUNTER     = ocgcore.TYPE_COUNTER     //
	TYPE_FLIP        = ocgcore.TYPE_FLIP        //
	TYPE_TOON        = ocgcore.TYPE_TOON        //
	TYPE_XYZ         = ocgcore.TYPE_XYZ         //
	TYPE_PENDULUM    = ocgcore.TYPE_PENDULUM    //
	TYPE_SPSUMMON    = ocgcore.TYPE_SPSUMMON    //
	TYPE_LINK        = ocgcore.TYPE_LINK        //

	TYPES_EXTRA_DECK = ocgcore.TYPES_EXTRA_DECK
)

func (dm *DataManager) GetData(code uint32) *ocgcore.CardData {
	data, has := datas[code]
	if has {
		return &data.CardData
	}
	return nil
}
