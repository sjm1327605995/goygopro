package main

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/sjm1327605995/goygopro/ocgcore"
)

// 波 G-2 搜索扩展的测试。不用 vendored cards.cdb（里面的真实卡名/效果
// 不可控），而是造一个只含已知测试卡的最小 cdb，让每条断言都确定。

type testCard struct {
	code     uint32
	alias    uint32
	setcode  uint64
	typ      uint32
	atk      int32
	def      int32
	level    uint32
	category int64
	name     string
	desc     string
}

var testCards = []testCard{
	// TYPE_MONSTER|TYPE_NORMAL=0x11；TYPE_MONSTER|TYPE_EFFECT=0x12
	{1001, 0, 0xdd, 0x11, 3000, 2500, 8, 0, "青眼白龙", "以高攻击力著称的龙。"},
	{1002, 0, 0, 0x12, 2500, 2100, 7, 0x1, "暗黑魔法师", "黑魔法的使者。"},
	{1003, 0, 0, 0x2, 0, 0, 0, 0x2, "增援", "从卡组把战士加入手卡。"},
	// LINK 卡：def 字段存 link_marker（↖0x40|↓0x2），level 低字节存连接数
	{1004, 0, 0, ocgcore.TYPE_MONSTER | ocgcore.TYPE_EFFECT | ocgcore.TYPE_LINK, 800, 0x42, 2, 0, "链接蜘蛛", "从网络那头爬来。"},
	// 灵摆：lscale=(level>>24)&0xff=3，rscale=(level>>16)&0xff=5
	{1005, 0, 0, ocgcore.TYPE_MONSTER | ocgcore.TYPE_PENDULUM | 0x10, 1800, 900, 0x03050004, 0, "灵摆龙", "在两个刻度间摇摆。"},
	{1006, 0, 0, 0x11, 2400, 1200, 7, 0, "红眼黑龙", "普通的龙。"},
	{1007, 0, 0, 0x11, 1000, 500, 4, 0, "魔法师", "陷阱 加入手卡"},
	{1008, 0, 0, 0x11, 900, 400, 3, 0, "陷阱大师", "陷阱加入手卡"},
	{1010, 0, 0, 0x11, 500, 300, 2, 0, "魔导师", "研究强大的龙。"},
	// 名字不含「青眼」但属于青眼系列（setcode 反查路径）
	{1011, 0, 0xdd, 0x11, 600, 400, 3, 0, "传说之龙", "被封印的牌。"},
	// alias 指向 1001：trycode 应按原卡号匹配（get_original_code）
	{1012, 1001, 0xdd, 0x11, 3000, 2500, 8, 0, "青眼白龙·异画", "另一种形态的白龙。"},
	// def=2 与链接箭头 ↓ 同值：验证守备过滤必须排除 LINK
	{1013, 0, 0, 0x11, 100, 2, 1, 0, "普通士兵", "站岗的士兵。"},
	// ?ATK 卡（-2）
	{1014, 0, 0, 0x11, -2, 0, 1, 0, "神秘之壁", "攻击力不明。"},
}

func newTestCardDB(t *testing.T) *CardDBManager {
	t.Helper()
	m := NewCardDBManager()
	// 共享内存库：Windows 下 TempDir 清理会因 sqlite 未关文件句柄而失败；
	// 名字带上测试名，避免共享缓存让各测试串库
	dsn := fmt.Sprintf("file:goygopro-card-search-%s?mode=memory&cache=shared", t.Name())
	if err := m.OpenDB(dsn); err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	schema := `CREATE TABLE datas (id INTEGER PRIMARY KEY, ot INTEGER, alias INTEGER, setcode INTEGER,
		type INTEGER, atk INTEGER, def INTEGER, level INTEGER, race INTEGER, attribute INTEGER, category INTEGER);
		CREATE TABLE texts (id INTEGER PRIMARY KEY, name TEXT, desc TEXT,
		str1 TEXT, str2 TEXT, str3 TEXT, str4 TEXT, str5 TEXT, str6 TEXT, str7 TEXT, str8 TEXT,
		str9 TEXT, str10 TEXT, str11 TEXT, str12 TEXT, str13 TEXT, str14 TEXT, str15 TEXT, str16 TEXT);`
	if _, err := m.db.Exec(schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	for _, c := range testCards {
		if _, err := m.db.Exec(`INSERT INTO datas VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			c.code, 1, c.alias, c.setcode, c.typ, c.atk, c.def, c.level, 0, 0, c.category); err != nil {
			t.Fatalf("insert datas %d: %v", c.code, err)
		}
		if _, err := m.db.Exec(`INSERT INTO texts VALUES (?,?,?, '', '', '', '', '', '', '', '', '', '', '', '', '', '', '', '')`,
			c.code, c.name, c.desc); err != nil {
			t.Fatalf("insert texts %d: %v", c.code, err)
		}
	}
	// 系列名表：真实 strings.conf 里 0xdd = 青眼
	if err := m.LoadSetNames(filepath.Join(findRepoRoot(t), "strings.conf")); err != nil {
		t.Fatalf("LoadSetNames: %v", err)
	}
	return m
}

func codes(res []CardInfo) map[uint32]bool {
	out := make(map[uint32]bool)
	for _, c := range res {
		out[c.Code] = true
	}
	return out
}

func expectCodes(t *testing.T, got map[uint32]bool, want ...uint32) {
	t.Helper()
	wantSet := make(map[uint32]bool)
	for _, c := range want {
		wantSet[c] = true
	}
	for c := range wantSet {
		if !got[c] {
			t.Errorf("缺少卡 %d，实际结果 %v", c, got)
		}
	}
	for c := range got {
		if !wantSet[c] {
			t.Errorf("多出卡 %d，期望 %v", c, want)
		}
	}
}

// 多关键词（空格分隔）= 全元素 AND（deck_con.cpp 匹配循环）
func TestSearchMultiKeywordsSpace(t *testing.T) {
	m := newTestCardDB(t)
	res := m.SearchCards(CardFilter{Keyword: "青眼 白龙", MultiKeywords: 1})
	expectCodes(t, codes(res), 1001, 1012)
}

// `-` 前缀排除（deck_con.cpp element.exclude）
func TestSearchExcludeElement(t *testing.T) {
	m := newTestCardDB(t)
	res := m.SearchCards(CardFilter{Keyword: "龙 -青眼", MultiKeywords: 1})
	// 1001/1011/1012 属青眼系列或名字含青眼 → 排除；1005/1006/1010 剩下
	expectCodes(t, codes(res), 1005, 1006, 1010)
}

// 引号短语把空格当字面量：1007 的 desc 有「陷阱 加入」（带空格），
// 1008 的「陷阱加入」不带——短语只命中前者，拆开则两者都命中
func TestSearchQuotedPhrase(t *testing.T) {
	m := newTestCardDB(t)
	phrase := m.SearchCards(CardFilter{Keyword: `"陷阱 加入"`, MultiKeywords: 1})
	expectCodes(t, codes(phrase), 1007)
	split := m.SearchCards(CardFilter{Keyword: "陷阱 加入", MultiKeywords: 1})
	expectCodes(t, codes(split), 1007, 1008)
}

// `$` 前缀仅卡名：1010 的「龙」只在 desc 里
func TestSearchNameOnlyDollar(t *testing.T) {
	m := newTestCardDB(t)
	expectCodes(t, codes(m.SearchCards(CardFilter{Keyword: "$龙", MultiKeywords: 1})), 1001, 1005, 1006, 1011, 1012)
	expectCodes(t, codes(m.SearchCards(CardFilter{Keyword: "龙", MultiKeywords: 1})), 1001, 1005, 1006, 1010, 1011, 1012)
}

// `@` 前缀仅系列名（setname 反查）；未知系列返回空
func TestSearchSetcodeAt(t *testing.T) {
	m := newTestCardDB(t)
	expectCodes(t, codes(m.SearchCards(CardFilter{Keyword: "@青眼", MultiKeywords: 1})), 1001, 1011, 1012)
	if res := m.SearchCards(CardFilter{Keyword: "@不存在的系列", MultiKeywords: 1}); len(res) != 0 {
		t.Fatalf("未知系列应无结果，得到 %v", codes(res))
	}
}

// 数字串按原卡号精确匹配（alias 优先，deck_con.cpp get_original_code）
func TestSearchTryCodeMatchesAlias(t *testing.T) {
	m := newTestCardDB(t)
	expectCodes(t, codes(m.SearchCards(CardFilter{Keyword: "1001"})), 1001, 1012)
}

// 单元素模式（multi=0）：整串一个关键词，空格不做分隔
func TestSearchSingleModeKeepsSpaces(t *testing.T) {
	m := newTestCardDB(t)
	if res := m.SearchCards(CardFilter{Keyword: "青眼 白龙", MultiKeywords: 0}); len(res) != 0 {
		t.Fatalf("单元素模式整串匹配应无结果，得到 %v", codes(res))
	}
	expectCodes(t, codes(m.SearchCards(CardFilter{Keyword: "青眼白龙", MultiKeywords: 0})), 1001, 1012)
}

// 效果类型位掩码：(category & effect) != 0（deck_con.cpp:1534）
func TestSearchEffectMask(t *testing.T) {
	m := newTestCardDB(t)
	expectCodes(t, codes(m.SearchCards(CardFilter{Effect: 0x1})), 1002)
	expectCodes(t, codes(m.SearchCards(CardFilter{Effect: 0x3})), 1002, 1003)
	if res := m.SearchCards(CardFilter{Effect: 0x4}); len(res) != 0 {
		t.Fatalf("无人命中位 0x4 应无结果，得到 %v", codes(res))
	}
}

// 链接箭头掩码：子集语义 + 必须 LINK（1013 的 def=2 不得混入）
func TestSearchLinkMarks(t *testing.T) {
	m := newTestCardDB(t)
	expectCodes(t, codes(m.SearchCards(CardFilter{LinkMarks: 0x2})), 1004)
	expectCodes(t, codes(m.SearchCards(CardFilter{LinkMarks: 0x40})), 1004)
	expectCodes(t, codes(m.SearchCards(CardFilter{LinkMarks: 0x42})), 1004)
	if res := m.SearchCards(CardFilter{LinkMarks: 0x1}); len(res) != 0 {
		t.Fatalf("无 ↙ 箭头卡应无结果，得到 %v", codes(res))
	}
}

// 灵摆刻度：要求 TYPE_PENDULUM，取 level 高位字节
func TestSearchScale(t *testing.T) {
	m := newTestCardDB(t)
	expectCodes(t, codes(m.SearchCards(CardFilter{ScaleFilter: "3"})), 1005)
	expectCodes(t, codes(m.SearchCards(CardFilter{ScaleFilter: ">=3"})), 1005)
	if res := m.SearchCards(CardFilter{ScaleFilter: ">=4"}); len(res) != 0 {
		t.Fatalf("刻度 ≥4 应无结果，得到 %v", codes(res))
	}
	minScale, maxScale := 2, 4
	expectCodes(t, codes(m.SearchCards(CardFilter{MinScale: &minScale, MaxScale: &maxScale})), 1005)
}

// 攻击力运算符字符串（parse_filter 翻译）
func TestSearchAtkOperator(t *testing.T) {
	m := newTestCardDB(t)
	expectCodes(t, codes(m.SearchCards(CardFilter{AtkFilter: ">=2500"})), 1001, 1002, 1012)
	expectCodes(t, codes(m.SearchCards(CardFilter{AtkFilter: "=3000"})), 1001, 1012)
	// 「?」只匹配 atk=-2
	expectCodes(t, codes(m.SearchCards(CardFilter{AtkFilter: "?"})), 1014)
	// op5 排除负值：1014（-2）不算「小于 1000」
	res := codes(m.SearchCards(CardFilter{AtkFilter: "<1000"}))
	if res[1014] {
		t.Errorf("?ATK(-2) 卡不应出现在 <1000 结果里")
	}
	if !res[1008] || !res[1004] {
		t.Errorf("<1000 应含 1008/1004，实际 %v", res)
	}
}

// 守备过滤排除 LINK（def 字段存的是箭头）
func TestSearchDefOperatorExcludesLink(t *testing.T) {
	m := newTestCardDB(t)
	res := codes(m.SearchCards(CardFilter{DefFilter: "=2"}))
	if !res[1013] {
		t.Errorf("def=2 的 1013 应命中，实际 %v", res)
	}
	if res[1004] {
		t.Errorf("LINK 卡（def 字段是箭头）不应参与守备过滤")
	}
}

// 星级：低字节比较；「?」恒不匹配（原版怪癖 1=0）
func TestSearchLevelOperator(t *testing.T) {
	m := newTestCardDB(t)
	expectCodes(t, codes(m.SearchCards(CardFilter{LevelFilter: "8"})), 1001, 1012)
	expectCodes(t, codes(m.SearchCards(CardFilter{LevelFilter: ">=7"})), 1001, 1002, 1006, 1012)
	if res := m.SearchCards(CardFilter{LevelFilter: "?"}); len(res) != 0 {
		t.Fatalf("星级「?」应恒不匹配（原版怪癖），得到 %v", codes(res))
	}
}
