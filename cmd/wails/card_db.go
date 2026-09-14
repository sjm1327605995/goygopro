package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/sjm1327605995/goygopro/ocgcore"
	_ "modernc.org/sqlite"
)

type CardInfo struct {
	Code       uint32   `json:"code"`
	Alias      uint32   `json:"alias"`
	Setcode    uint64   `json:"setcode"`
	Type       uint32   `json:"type"`
	Attack     int32    `json:"attack"`
	Defense    int32    `json:"defense"`
	Level      uint32   `json:"level"`
	Race       uint32   `json:"race"`
	Attribute  uint32   `json:"attribute"`
	Category   int64    `json:"category"`
	Name       string   `json:"name"`
	Desc       string   `json:"desc"`
	Strings    []string `json:"strings"`
	SetNames   []string `json:"setNames"`
	LScale     uint32   `json:"lscale"`
	RScale     uint32   `json:"rscale"`
	LinkMarker uint32   `json:"linkMarker"`
	Ot         uint32   `json:"ot"`
}

// CardFilter 的 Min/Max 字段用指针表达「未设置」：JSON 缺省/0 值区分不开
// 「攻 0」和「不限攻」，此前 `MinAttack >= 0` 恒真导致 ?ATK(-2) 卡被默默
// 排除。前端不传即 nil；?ATK 用 -2/-2 区间检索。
//
// 波 G-2（deck_con.cpp FilterCards 1402-1600 的翻译）：
//   - Atk/Def/Level/ScaleFilter 收 gframe 风格的运算符字符串（parse_filter
//     13-42：`=`/裸数字=相等、`>=`/`>`/`<=`/`<`、`?`=「?」占位卡）
//   - Effect 是 wCategories 32 复选框汇出的 category 位掩码
//   - LinkMarks 是 wLinkMarks 8 箭头汇出的 link_marker 位掩码（cdb 里存
//     在 LINK 卡的 def 字段，因此必须同时要求 TYPE_LINK）
//   - Min/MaxScale 要求灵摆，刻度取 level 高位字节
type CardFilter struct {
	Keyword     string `json:"keyword"`
	Type        uint32 `json:"type"`
	Race        uint32 `json:"race"`
	Attribute   uint32 `json:"attribute"`
	Level       int    `json:"level"`
	MinLevel    *int   `json:"minLevel"`
	MaxLevel    *int   `json:"maxLevel"`
	MinAttack   *int   `json:"minAttack"`
	MaxAttack   *int   `json:"maxAttack"`
	MinDefense  *int   `json:"minDefense"`
	MaxDefense  *int   `json:"maxDefense"`
	AtkFilter   string `json:"atkFilter"`
	DefFilter   string `json:"defFilter"`
	LevelFilter string `json:"levelFilter"`
	ScaleFilter string `json:"scaleFilter"`
	Effect      uint64 `json:"effect"`
	LinkMarks   uint32 `json:"linkMarks"`
	MinScale    *int   `json:"minScale"`
	MaxScale    *int   `json:"maxScale"`
	// MultiKeywords：0=单关键词（整串一个元素）、1=空格分隔、2=加号分隔
	// （gframe gameConf.search_multiple_keywords）。缺省 0 保持旧行为。
	MultiKeywords int `json:"multiKeywords"`
	Limit         int `json:"limit"`
	Offset        int `json:"offset"`
}

type DeckData struct {
	Name  string   `json:"name"`
	Main  []uint32 `json:"main"`
	Extra []uint32 `json:"extra"`
	Side  []uint32 `json:"side"`
}

type CardDBManager struct {
	mu       sync.RWMutex
	db       *sqlx.DB
	cache    map[uint32]*CardInfo
	setNames *SetNameTable
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

// LoadSetNames 从 strings.conf 读取系列名表（可选数据，缺文件不报错）。
func (m *CardDBManager) LoadSetNames(path string) error {
	if m.setNames == nil {
		m.setNames = NewSetNameTable()
	}
	return m.setNames.Load(path)
}

func (m *CardDBManager) GetCard(code uint32) *CardInfo {
	// 0x80000000 是协议里的公开标记（如表侧抽卡），不是卡号的一部分
	code &^= 0x80000000
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
	cd.SetNames = m.formatSetNames(cd.Setcode)

	m.mu.Lock()
	m.cache[code] = &cd
	m.mu.Unlock()
	return &cd
}

// formatSetNames 展开系列名；表未加载时返回 nil（调用点在锁外）。
func (m *CardDBManager) formatSetNames(setcode uint64) []string {
	m.mu.RLock()
	t := m.setNames
	m.mu.RUnlock()
	if t == nil {
		return nil
	}
	return t.FormatSetNames(setcode)
}

// gframe FilterCards 的比较语义（deck_con.cpp:1477-1523）：op1=相等、
// op2=≥、op3=＞、op4=≤、op5=＜、op6=「?」占位。op4/op5 会把负值（? 卡的
// -2）一并排除；atk/def 的 op6 只匹配 -2，level/scale 的 op6 恒不匹配
// （原版直接 continue，是个忠实保留的怪癖）。
const (
	filterEqual        = 1
	filterGreaterEqual = 2
	filterGreater      = 3
	filterLessEqual    = 4
	filterLess         = 5
	filterUndefined    = 6
)

// parseStatFilter 翻译 deck_con.cpp:13-42 parse_filter：`=` 或裸数字=相等、
// `>=`/`>`/`<=`/`<` 为比较、`?` 为占位查询。解析失败返回 ok=false（不过滤）。
func parseStatFilter(p string) (op int, value int, ok bool) {
	p = strings.TrimSpace(p)
	if p == "" {
		return 0, 0, false
	}
	if p == "?" {
		return filterUndefined, 0, true
	}
	op = filterEqual
	switch {
	case strings.HasPrefix(p, ">="):
		op, p = filterGreaterEqual, p[2:]
	case strings.HasPrefix(p, "<="):
		op, p = filterLessEqual, p[2:]
	case strings.HasPrefix(p, ">"):
		op, p = filterGreater, p[1:]
	case strings.HasPrefix(p, "<"):
		op, p = filterLess, p[1:]
	case strings.HasPrefix(p, "="):
		op, p = filterEqual, p[1:]
	}
	v, err := strconv.Atoi(strings.TrimSpace(p))
	if err != nil {
		return 0, 0, false
	}
	return op, v, true
}

// statOpCondition 把 op 翻成 expr 上的 SQL 片段。op4/op5 排除负值
// （gframe 的 `data.attack < 0` continue）；op6 交给调用方按字段语义处理。
func statOpCondition(expr string, op, value int) (string, interface{}) {
	switch op {
	case filterEqual:
		return expr + " = ?", value
	case filterGreaterEqual:
		return expr + " >= ?", value
	case filterGreater:
		return expr + " > ?", value
	case filterLessEqual:
		return fmt.Sprintf("(%s >= 0 AND %s <= ?)", expr, expr), value
	case filterLess:
		return fmt.Sprintf("(%s >= 0 AND %s < ?)", expr, expr), value
	}
	return "1 = 0", nil
}

// kwElement 是一个搜索元素（deck_con.cpp element_t）。
type kwElement struct {
	keyword  string
	setcodes []uint32
	kind     int // 0=all 1=name 2=setcode
	exclude  bool
}

// parseKeywordElements 翻译 deck_con.cpp:1417-1477：multi=1 空格分隔、
// 2=加号分隔、0=单元素（整串一个元素，不支持 -/引号）。元素前缀 `-` 排除、
// `$` 仅卡名、`@` 仅系列；引号短语把分隔符当字面量。
// ASCII 分隔符不会出现在 UTF-8 多字节序列里，按字节切是安全的。
func parseKeywordElements(kw string, multi int, table *SetNameTable) []kwElement {
	if strings.TrimSpace(kw) == "" {
		return nil
	}
	sep := byte(' ')
	if multi == 2 {
		sep = '+'
	}
	var elements []kwElement
	i := 0
	for i < len(kw) {
		for i < len(kw) && kw[i] == sep {
			i++
		}
		if i >= len(kw) {
			break
		}
		var el kwElement
		if multi != 0 && kw[i] == '-' {
			el.exclude = true
			i++
			if i >= len(kw) {
				break
			}
		}
		if kw[i] == '$' {
			el.kind = 1
			i++
		} else if kw[i] == '@' {
			el.kind = 2
			i++
		}
		if i >= len(kw) {
			break
		}
		if multi == 0 {
			// 单元素模式：余下整串都是关键词（含空格）
			el.keyword = kw[i:]
			if el.kind == 2 && table != nil {
				el.setcodes = table.CodesByName(el.keyword)
			}
			elements = append(elements, el)
			break
		}
		delim := sep
		if kw[i] == '"' {
			delim = '"'
			i++
		}
		end := strings.IndexByte(kw[i:], delim)
		if end >= 0 {
			el.keyword = kw[i : i+end]
			i += end + 1
		} else {
			el.keyword = kw[i:]
			i = len(kw)
		}
		if el.kind == 2 && table != nil {
			el.setcodes = table.CodesByName(el.keyword)
		}
		elements = append(elements, el)
	}
	return elements
}

func (m *CardDBManager) SearchCards(f CardFilter) []CardInfo {
	if m.db == nil {
		return nil
	}

	query := `SELECT datas.id, ot, alias, setcode, type, atk, def, level, race, attribute, category, name, desc
		FROM datas JOIN texts ON datas.id = texts.id WHERE 1=1`
	var args []interface{}

	m.mu.RLock()
	table := m.setNames
	m.mu.RUnlock()

	// trycode：整串能解析成数字时按原卡号（alias 优先）精确匹配
	// （deck_con.cpp:1407 BufferIO::GetVal + get_original_code）
	trycode := 0
	if v, err := strconv.Atoi(strings.TrimSpace(f.Keyword)); err == nil && v > 0 {
		trycode = v
	}

	// 关键字元素（deck_con.cpp:1417-1477 与匹配循环 1560-1580 的翻译）：
	//   $xxx 仅卡名；@xxx 仅系列名（setname 反查）；其余 名称/效果/卡号/
	//   系列；`-` 前缀取反；全元素 AND
	for _, el := range parseKeywordElements(f.Keyword, f.MultiKeywords, table) {
		if el.keyword == "" {
			continue
		}
		var elemCond string
		switch el.kind {
		case 1:
			elemCond = "name LIKE ?"
			args = append(args, "%"+el.keyword+"%")
		case 2:
			if len(el.setcodes) == 0 {
				// 系列码反查为空：非排除元素必不匹配，直接无结果；
				// 排除元素恒真，跳过
				if !el.exclude {
					return nil
				}
				continue
			}
			var conds []string
			for _, c := range el.setcodes {
				for shift := 0; shift < 64; shift += 16 {
					conds = append(conds, fmt.Sprintf("((setcode >> %d) & 0xffff) = ?", shift))
					args = append(args, c)
				}
			}
			elemCond = "(" + strings.Join(conds, " OR ") + ")"
		default:
			var conds []string
			conds = append(conds, "name LIKE ?", "desc LIKE ?", "datas.id LIKE ?")
			pattern := "%" + el.keyword + "%"
			args = append(args, pattern, pattern, pattern)
			if trycode > 0 {
				conds = append(conds, "(CASE WHEN alias != 0 THEN alias ELSE datas.id END) = ?")
				args = append(args, trycode)
			}
			if table != nil {
				if codes := table.CodesByName(el.keyword); len(codes) > 0 {
					var setConds []string
					for _, c := range codes {
						for shift := 0; shift < 64; shift += 16 {
							setConds = append(setConds, fmt.Sprintf("((setcode >> %d) & 0xffff) = ?", shift))
							args = append(args, c)
						}
					}
					conds = append(conds, "("+strings.Join(setConds, " OR ")+")")
				}
			}
			elemCond = "(" + strings.Join(conds, " OR ") + ")"
		}
		if el.exclude {
			query += " AND NOT " + elemCond
		} else {
			query += " AND " + elemCond
		}
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
	if f.MinLevel != nil && *f.MinLevel > 0 {
		query += " AND (level & 0xff) >= ?"
		args = append(args, *f.MinLevel)
	}
	if f.MaxLevel != nil && *f.MaxLevel > 0 {
		query += " AND (level & 0xff) <= ?"
		args = append(args, *f.MaxLevel)
	}
	if f.MinAttack != nil {
		query += " AND atk >= ?"
		args = append(args, *f.MinAttack)
	}
	if f.MaxAttack != nil {
		query += " AND atk <= ?"
		args = append(args, *f.MaxAttack)
	}
	if f.MinDefense != nil {
		query += " AND def >= ?"
		args = append(args, *f.MinDefense)
	}
	if f.MaxDefense != nil {
		query += " AND def <= ?"
		args = append(args, *f.MaxDefense)
	}

	// ---- 波 G-2：gframe 运算符字符串过滤（deck_con.cpp:1477-1523）----
	// 守备过滤一律排除 LINK（cdb 的 def 字段存的是连接箭头，不是守备）
	defFiltered := f.MinDefense != nil || f.MaxDefense != nil
	if op, v, ok := parseStatFilter(f.AtkFilter); ok {
		switch op {
		case filterUndefined:
			query += " AND atk = -2"
		default:
			cond, arg := statOpCondition("atk", op, v)
			query += " AND " + cond
			args = append(args, arg)
		}
	}
	if op, v, ok := parseStatFilter(f.DefFilter); ok {
		defFiltered = true
		switch op {
		case filterUndefined:
			query += " AND def = -2"
		default:
			cond, arg := statOpCondition("def", op, v)
			query += " AND " + cond
			args = append(args, arg)
		}
	}
	if defFiltered {
		query += " AND (type & ?) = 0"
		args = append(args, ocgcore.TYPE_LINK)
	}
	if op, v, ok := parseStatFilter(f.LevelFilter); ok {
		switch op {
		case filterUndefined:
			query += " AND 1 = 0" // 原版怪癖：星级过滤的「?」恒不匹配
		default:
			cond, arg := statOpCondition("(level & 0xff)", op, v)
			query += " AND " + cond
			args = append(args, arg)
		}
	}
	// 灵摆刻度：要求 TYPE_PENDULUM，刻度在 level 高位字节；「?」恒不匹配
	applyScale := func(s string) {
		if op, v, ok := parseStatFilter(s); ok {
			query += " AND (type & ?) != 0"
			args = append(args, ocgcore.TYPE_PENDULUM)
			switch op {
			case filterUndefined:
				query += " AND 1 = 0"
			default:
				cond, arg := statOpCondition("((level >> 24) & 0xff)", op, v)
				query += " AND " + cond
				args = append(args, arg)
			}
		}
	}
	applyScale(f.ScaleFilter)
	if f.MinScale != nil || f.MaxScale != nil {
		query += " AND (type & ?) != 0"
		args = append(args, ocgcore.TYPE_PENDULUM)
		if f.MinScale != nil {
			query += " AND ((level >> 24) & 0xff) >= ?"
			args = append(args, *f.MinScale)
		}
		if f.MaxScale != nil {
			query += " AND ((level >> 24) & 0xff) <= ?"
			args = append(args, *f.MaxScale)
		}
	}

	// 效果类型位掩码（wCategories 32 复选框 → BUTTON_CATEGORY_OK）：
	// (category & filter) 任一位命中即通过（deck_con.cpp:1534）
	if f.Effect != 0 {
		query += " AND (category & ?) != 0"
		args = append(args, int64(f.Effect))
	}
	// 链接箭头掩码（wLinkMarks → BUTTON_MARKERS_OK）：子集语义
	// (link_marker & marks) == marks；link_marker 存在 LINK 卡的 def 字段
	if f.LinkMarks != 0 {
		query += " AND (type & ?) != 0 AND (def & ?) = ?"
		args = append(args, ocgcore.TYPE_LINK, f.LinkMarks, f.LinkMarks)
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
			cd.SetNames = m.formatSetNames(cd.Setcode)
			results = append(results, cd)
		}
	}
	return results
}

// ------------------------------------------------------------------
// Deck Management (.ydk format)
// ------------------------------------------------------------------

// ListDecks 递归列出 deckDir 下的 .ydk（gframe 分类下拉的 ./deck/<分类>/
// 子目录语义，game.cpp:1274 TraversalDir isdir）。返回去掉 .ydk 后缀的
// 相对路径名，'/' 分隔分类层级；根目录的卡组即「未分类」。
func (m *CardDBManager) ListDecks(deckDir string) []string {
	if _, err := os.Stat(deckDir); os.IsNotExist(err) {
		_ = os.MkdirAll(deckDir, 0755)
		return nil
	}
	var names []string
	err := filepath.WalkDir(deckDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 读不了的子目录跳过，不影响其余卡组
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".ydk") {
			return nil
		}
		rel, rerr := filepath.Rel(deckDir, path)
		if rerr != nil {
			return nil
		}
		names = append(names, strings.TrimSuffix(filepath.ToSlash(rel), ".ydk"))
		return nil
	})
	if err != nil {
		return nil
	}
	return names
}

// safeDeckPath：卡组名可含 '/'（分类/子分类/名），但只允许落在 deckDir
// 内——拒绝空名、绝对路径、'..'、空段等穿越写法。
func safeDeckPath(deckDir, name string) (string, error) {
	name = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(name), ".ydk"))
	if name == "" {
		return "", errors.New("卡组名不能为空")
	}
	if filepath.IsAbs(name) || filepath.IsAbs(filepath.FromSlash(name)) {
		return "", fmt.Errorf("非法卡组名：%q", name)
	}
	segs := strings.Split(filepath.ToSlash(name), "/")
	for _, s := range segs {
		if s == "" || s == "." || s == ".." || strings.ContainsAny(s, `\/:`) {
			return "", fmt.Errorf("非法卡组名：%q", name)
		}
	}
	// VolumeName 兜底 Windows 盘符（"C:" 已被上面的 ':' 拒掉，双保险）
	if filepath.VolumeName(filepath.FromSlash(strings.Join(segs, "/"))) != "" {
		return "", fmt.Errorf("非法卡组名：%q", name)
	}
	return filepath.Join(deckDir, filepath.FromSlash(strings.Join(segs, "/"))+".ydk"), nil
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
