package main

import (
	"database/sql"
	"unicode/utf16"

	"github.com/jmoiron/sqlx"
)

const (
	SIZE_SETCODE = 16
	TYPE_LINK    = 0x4000000
)

type CardString struct {
	Name string   `db:"name"`
	Text string   `db:"text"`
	Desc []string `db:"desc"`
}

type CardData struct {
	Code       uint32 `db:"id"`
	OT         int    `db:"ot"`
	Alias      uint32 `db:"alias"`
	Setcode    [SIZE_SETCODE]uint16
	Type       uint32 `db:"type"`
	Attack     int32  `db:"atk"`
	Defense    int32  `db:"def"`
	Level      uint8
	LScale     uint8
	RScale     uint8
	Race       uint32 `db:"race"`
	Attribute  uint32 `db:"attribute"`
	Category   int64  `db:"category"`
	LinkMarker uint32
}

// SetSetCode 设置setcode数组
func (c *CardData) SetSetCode(value uint64) {
	ctr := 0
	for value != 0 {
		if value&0xffff != 0 {
			c.Setcode[ctr] = uint16(value & 0xffff)
			ctr++
		}
		value >>= 16
	}
	for i := ctr; i < SIZE_SETCODE; i++ {
		c.Setcode[i] = 0
	}
}

type DataManager struct {
	db           *sqlx.DB
	datas        map[uint32]CardData
	strings      map[uint32]CardString
	extraSetcode map[uint32][]uint16
}

func NewDataManager(db *sqlx.DB) *DataManager {
	return &DataManager{
		db:      db,
		datas:   make(map[uint32]CardData, 32768),
		strings: make(map[uint32]CardString, 32768),
		extraSetcode: map[uint32][]uint16{
			8512558: {0x8f, 0x54, 0x59, 0x82, 0x13a},
		},
	}
}
func (dm *DataManager) GetCardData(code uint32) (CardData, bool) {
	data, ok := dm.datas[code]
	return data, ok
}
func (dm *DataManager) ReadDB() error {
	type dbRow struct {
		CardData
		SetcodeRaw uint64         `db:"setcode"`
		LevelRaw   uint32         `db:"level"`
		Name       sql.NullString `db:"name"`
		Text       sql.NullString `db:"desc"`
		// 可以添加更多 desc 字段
	}

	var rows []dbRow
	query := `SELECT 
		datas.id, datas.ot, datas.alias, datas.setcode, 
		datas.type, datas.atk, datas.def, datas.level,
		datas.race, datas.attribute, datas.category,
		texts.name, texts.desc
		FROM datas, texts WHERE datas.id = texts.id`

	if err := dm.db.Select(&rows, query); err != nil {
		return err
	}

	for _, row := range rows {
		// 处理 setcode
		if row.SetcodeRaw != 0 {
			if extra, ok := dm.extraSetcode[row.Code]; ok {
				copy(row.Setcode[:], extra)
			} else {
				row.CardData.SetSetCode(row.SetcodeRaw)
			}
		}

		// 处理 level 数据
		row.Level = uint8(row.LevelRaw & 0xff)
		row.LScale = uint8((row.LevelRaw >> 24) & 0xff)
		row.RScale = uint8((row.LevelRaw >> 16) & 0xff)

		// 处理 link marker
		if row.Type&TYPE_LINK != 0 {
			row.LinkMarker = uint32(row.Defense)
			row.Defense = 0
		} else {
			row.LinkMarker = 0
		}

		// 存储数据
		dm.datas[row.Code] = row.CardData

		// 处理字符串
		cs := CardString{}
		if row.Name.Valid {
			cs.Name = row.Name.String
		}
		if row.Text.Valid {
			cs.Text = row.Text.String
		}
		// 可以添加 desc 处理
		dm.strings[row.Code] = cs
	}

	return nil
}

// UTF-8 到 UTF-16 转换辅助函数
func decodeUTF8(s string) []uint16 {
	return utf16.Encode([]rune(s))
}
