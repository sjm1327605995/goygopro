package duel

import (
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
	"github.com/sjm1327605995/goygopro/ocgcore"
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

func (d *DataManager) LoadDB(file string) error {
	db, err := sqlx.Open("sqlite", file)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	raws, err := db.Query("select datas.id, ot, alias ,setcode ,type ,atk, def, level, race ,attribute, category ,name, desc," +
		"str1, str2, str3 ,str4 ,str5, str6, str7, str8, str9, str10, str11, str12 ,str13, str14,str15 ,str16 from datas,texts where datas.id=texts.id")
	if err != nil {
		return err
	}
	for raws.Next() {
		var (
			cd       CardDataC
			cs       CardString
			setCode  uint64
			level    uint32
			scanList = []any{&cd.Code, &cd.Ot, &cd.Alias, &setCode, &cd.Type, &cd.Attack, &cd.Defense, &level, &cd.Race, &cd.Attribute,
				&cd.Category, &cs.Name, &cs.Text}
		)
		for i := range cs.Desc {
			scanList = append(scanList, &cs.Desc[i])
		}
		err = raws.Scan(scanList...)
		if err != nil {
			return err
		}
		cd.SetSetCode(setCode)
		// rule_code 推导（data_manager.cpp:61-68）：5405695 特例与其余卡的
		// 非异画 alias 一律迁入 rule_code 并清零 alias。
		if cd.Code == ocgcore.CARD_BLACK_LUSTER_SOLDIER2 {
			cd.RuleCode = cd.Alias
			cd.Alias = 0
		} else if cd.Alias != 0 && cd.Type&ocgcore.TYPE_TOKEN == 0 && !isAlternativeCode(cd.Code, cd.Alias) {
			cd.RuleCode = cd.Alias
			cd.Alias = 0
		}
		if cd.Type&ocgcore.TYPE_LINK != 0 {
			cd.LinkMarker = uint32(cd.Defense)
			cd.Defense = 0
		} else {
			cd.LinkMarker = 0
		}
		cd.Level = level & 0xff
		cd.LScale = (level >> 24) & 0xff
		cd.RScale = (level >> 16) & 0xff
		datas[cd.Code] = &cd
		stringPointer[cd.Code] = &cs
	}
	// 全表后处理（data_manager.cpp:86-104）：
	// 1) 仍带 alias 的卡（异画等）继承 alias 目标的 rule_code；
	// 2) extra_setcode 无条件覆盖对应卡的系列码。
	for _, cd := range datas {
		if cd.RuleCode != 0 || cd.Alias == 0 || cd.Type&ocgcore.TYPE_TOKEN != 0 {
			continue
		}
		if target, ok := datas[cd.Alias]; ok {
			cd.RuleCode = target.RuleCode
		}
	}
	for code, list := range extraSetCode {
		cd, ok := datas[code]
		if !ok || len(list) == 0 || len(list) > ocgcore.SIZE_SETCODE {
			continue
		}
		copy(cd.Setcode[:], list)
	}
	return nil
}

// isAlternativeCode 对应原版 is_alternative（data_manager.cpp:19-21）：
// 互为异画的卡号（ artwork 版本偏移内）不做 rule_code 迁移。
func isAlternativeCode(code, alias uint32) bool {
	offset := uint32(ocgcore.CARD_ARTWORK_VERSIONS_OFFSET)
	return alias != 0 && alias < code+offset && code < alias+offset
}
func (d *DataManager) GetCodePointer(code uint32) *CardDataC {
	return datas[code]
}

const (
	//Types
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

func (d *DataManager) GetData(code uint32) *ocgcore.CardData {
	data, has := datas[code]
	if has {
		return &data.CardData
	}
	return nil
}
