package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
	"github.com/sjm1327605995/goygopro/ocgcore"
)

type CardInfo struct {
	Code        uint32   `json:"code"`
	Alias       uint32   `json:"alias"`
	Setcode     uint64   `json:"setcode"`
	Type        uint32   `json:"type"`
	Attack      int32    `json:"attack"`
	Defense     int32    `json:"defense"`
	Level       uint32   `json:"level"`
	Race        uint32   `json:"race"`
	Attribute   uint32   `json:"attribute"`
	Category    int64    `json:"category"`
	Name        string   `json:"name"`
	Desc        string   `json:"desc"`
	Strings     []string `json:"strings"`
	LScale      uint32   `json:"lscale"`
	RScale      uint32   `json:"rscale"`
	LinkMarker  uint32   `json:"linkMarker"`
	Ot          uint32   `json:"ot"`
}

type CardFilter struct {
	Keyword    string `json:"keyword"`
	Type       uint32 `json:"type"`
	Race       uint32 `json:"race"`
	Attribute  uint32 `json:"attribute"`
	Level      int    `json:"level"`
	MinAttack  int    `json:"minAttack"`
	MaxAttack  int    `json:"maxAttack"`
	MinDefense int    `json:"minDefense"`
	MaxDefense int    `json:"maxDefense"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
}

type DeckData struct {
	Name  string   `json:"name"`
	Main  []uint32 `json:"main"`
	Extra []uint32 `json:"extra"`
	Side  []uint32 `json:"side"`
}

type CardDBManager struct {
	mu    sync.RWMutex
	db    *sqlx.DB
	cache map[uint32]*CardInfo
}

func NewCardDBManager() *CardDBManager {
	return &CardDBManager{
		cache: make(map[uint32]*CardInfo),
	}
}

func (m *CardDBManager) OpenDB(dbPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	db, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	m.db = db
	return nil
}

func (m *CardDBManager) GetCard(code uint32) *CardInfo {
	m.mu.RLock()
	if card, ok := m.cache[code]; ok {
		m.mu.RUnlock()
		return card
	}
	m.mu.RUnlock()

	if m.db == nil {
		return nil
	}

	query := `SELECT datas.id, ot, alias, setcode, type, atk, def, level, race, attribute, category, name, desc,
		str1, str2, str3, str4, str5, str6, str7, str8, str9, str10, str11, str12, str13, str14, str15, str16
		FROM datas JOIN texts ON datas.id = texts.id WHERE datas.id = ?`

	var (
		cd        CardInfo
		strList   [16]string
		levelInfo uint32
	)

	row := m.db.QueryRow(query, code)
	err := row.Scan(&cd.Code, &cd.Ot, &cd.Alias, &cd.Setcode, &cd.Type, &cd.Attack, &cd.Defense, &levelInfo,
		&cd.Race, &cd.Attribute, &cd.Category, &cd.Name, &cd.Desc,
		&strList[0], &strList[1], &strList[2], &strList[3], &strList[4], &strList[5], &strList[6], &strList[7],
		&strList[8], &strList[9], &strList[10], &strList[11], &strList[12], &strList[13], &strList[14], &strList[15])
	if err != nil {
		return nil
	}

	cd.Level = levelInfo & 0xff
	cd.LScale = (levelInfo >> 24) & 0xff
	cd.RScale = (levelInfo >> 16) & 0xff
	if cd.Type&ocgcore.TYPE_LINK != 0 {
		cd.LinkMarker = uint32(cd.Defense)
		cd.Defense = 0
	}
	cd.Strings = strList[:]

	m.mu.Lock()
	m.cache[code] = &cd
	m.mu.Unlock()
	return &cd
}

func (m *CardDBManager) SearchCards(f CardFilter) []CardInfo {
	if m.db == nil {
		return nil
	}

	query := `SELECT datas.id, ot, alias, setcode, type, atk, def, level, race, attribute, category, name, desc
		FROM datas JOIN texts ON datas.id = texts.id WHERE 1=1`
	var args []interface{}

	if f.Keyword != "" {
		query += " AND (name LIKE ? OR desc LIKE ? OR datas.id LIKE ?)"
		pattern := "%" + f.Keyword + "%"
		args = append(args, pattern, pattern, pattern)
	}
	if f.Type != 0 {
		query += " AND (type & ?) = ?"
		args = append(args, f.Type, f.Type)
	}
	if f.Race != 0 {
		query += " AND (race & ?) != 0"
		args = append(args, f.Race)
	}
	if f.Attribute != 0 {
		query += " AND (attribute & ?) != 0"
		args = append(args, f.Attribute)
	}
	if f.Level > 0 {
		query += " AND (level & 0xff) = ?"
		args = append(args, f.Level)
	}
	if f.MinAttack >= 0 {
		query += " AND atk >= ?"
		args = append(args, f.MinAttack)
	}
	if f.MaxAttack >= 0 {
		query += " AND atk <= ?"
		args = append(args, f.MaxAttack)
	}
	if f.MinDefense >= 0 {
		query += " AND def >= ?"
		args = append(args, f.MinDefense)
	}
	if f.MaxDefense >= 0 {
		query += " AND def <= ?"
		args = append(args, f.MaxDefense)
	}

	limit := 50
	if f.Limit > 0 && f.Limit <= 200 {
		limit = f.Limit
	}
	query += " LIMIT ? OFFSET ?"
	args = append(args, limit, f.Offset)

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []CardInfo
	for rows.Next() {
		var cd CardInfo
		var levelInfo uint32
		err := rows.Scan(&cd.Code, &cd.Ot, &cd.Alias, &cd.Setcode, &cd.Type, &cd.Attack, &cd.Defense, &levelInfo,
			&cd.Race, &cd.Attribute, &cd.Category, &cd.Name, &cd.Desc)
		if err == nil {
			cd.Level = levelInfo & 0xff
			cd.LScale = (levelInfo >> 24) & 0xff
			cd.RScale = (levelInfo >> 16) & 0xff
			if cd.Type&ocgcore.TYPE_LINK != 0 {
				cd.LinkMarker = uint32(cd.Defense)
				cd.Defense = 0
			}
			results = append(results, cd)
		}
	}
	return results
}

// ------------------------------------------------------------------
// Deck Management (.ydk format)
// ------------------------------------------------------------------

func (m *CardDBManager) ListDecks(deckDir string) []string {
	if _, err := os.Stat(deckDir); os.IsNotExist(err) {
		_ = os.MkdirAll(deckDir, 0755)
		return nil
	}
	files, err := os.ReadDir(deckDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), ".ydk") {
			names = append(names, strings.TrimSuffix(f.Name(), filepath.Ext(f.Name())))
		}
	}
	return names
}

func (m *CardDBManager) LoadDeck(deckPath string) (*DeckData, error) {
	file, err := os.Open(deckPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	deck := &DeckData{
		Name: strings.TrimSuffix(filepath.Base(deckPath), filepath.Ext(deckPath)),
	}

	section := 0 // 1: main, 2: extra, 3: side
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#main") {
			section = 1
			continue
		} else if strings.HasPrefix(line, "#extra") {
			section = 2
			continue
		} else if strings.HasPrefix(line, "!side") {
			section = 3
			continue
		} else if strings.HasPrefix(line, "#") {
			continue
		}

		if line == "" {
			continue
		}

		code, err := strconv.ParseUint(line, 10, 32)
		if err != nil {
			continue
		}

		switch section {
		case 1:
			deck.Main = append(deck.Main, uint32(code))
		case 2:
			deck.Extra = append(deck.Extra, uint32(code))
		case 3:
			deck.Side = append(deck.Side, uint32(code))
		}
	}
	return deck, nil
}

func (m *CardDBManager) SaveDeck(deckPath string, deck DeckData) error {
	dir := filepath.Dir(deckPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0755)
	}

	file, err := os.Create(deckPath)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	_, _ = w.WriteString("#created by goygopro\n#main\n")
	for _, code := range deck.Main {
		_, _ = w.WriteString(fmt.Sprintf("%d\n", code))
	}
	_, _ = w.WriteString("#extra\n")
	for _, code := range deck.Extra {
		_, _ = w.WriteString(fmt.Sprintf("%d\n", code))
	}
	_, _ = w.WriteString("!side\n")
	for _, code := range deck.Side {
		_, _ = w.WriteString(fmt.Sprintf("%d\n", code))
	}
	return w.Flush()
}
