package duel

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sjm1327605995/goygopro/ocgcore"
)

const MAX_STRING_ID = 0x7ff
const MIN_CARD_ID = uint32((MAX_STRING_ID + 1) >> 4)
const MAX_CARD_ID = uint32(0x0fffffff)

var (
	stringPointer = make(map[uint32]*CardString, 32768)
	extraSetCode  = make(map[uint32][]uint16)
	datas         = make(map[uint32]*CardDataC, 32768)
)
var DefaultDataManager = new(DataManager)

type DataManager struct {
}

func (d *DataManager) LoadDB(file string) error {
	db, err := sqlx.Open("sqlite3", file)
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
		if setCode != 0 {
			it, exist := extraSetCode[cd.Code]
			if exist {
				var setCodeLen = len(it)
				if setCodeLen > ocgcore.SIZE_SETCODE {
					setCodeLen = ocgcore.SIZE_SETCODE
				}
				if setCodeLen != 0 {
					copy(cd.Setcode[:], it[:setCodeLen])
				} else {
					cd.SetSetCode(setCode)
				}
			}
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
	return nil
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
		fmt.Println("type=======>", data.Type, !(data.Type&TYPE_NORMAL != 0) || (data.Type&TYPE_PENDULUM != 0))
		return &data.CardData
	}
	return nil
}
