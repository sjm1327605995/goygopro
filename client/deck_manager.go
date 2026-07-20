package client

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// LFList corresponds to C++ struct LFList in deck_manager.h
type LFList struct {
	Hash     uint32
	ListName string
	Content  map[uint32]int
}

// Deck corresponds to C++ struct Deck in deck.h
type Deck struct {
	Main  []uint32
	Extra []uint32
	Side  []uint32
}

// DeckArray corresponds to C++ struct DeckArray in deck.h
type DeckArray struct {
	Main  []uint32
	Extra []uint32
	Side  []uint32
}

// DeckManager corresponds to C++ class DeckManager in deck_manager.h
type DeckManager struct {
	CurrentDeck Deck
	LFLists     []LFList
}

var DeckMgr = &DeckManager{}

// LoadLFList 读入禁卡表（lflist.conf）。
//
// 段落头是 `!名称`，不是 `[名称]` —— 此前认错了标记，于是**一个禁卡表都载不进来**，
// 卡组的禁限校验形同虚设。
//
// hash 尤其要紧：它是服务端匹配禁卡表的键（房间里选了哪个表就发哪个 hash），
// 算法必须与 C++ 逐位一致，否则两边对不上号。种子 0x7dfcee6a 与每条的异或式
// 都照抄 deck_manager.cpp 的 LoadLFListSingle。
func (dm *DeckManager) LoadLFList() {
	// 顺序与 C++ 一致：先扩展包的，再根目录的，最后补一个「无限制」。
	dm.loadLFListSingle("expansions/lflist.conf")
	dm.loadLFListSingle("lflist.conf")
	// N/A 是「不套用任何禁卡表」，hash 固定为 0。少了它，房间里就没法选「无限制」，
	// 而且表的下标会跟服务端整体错开一位。
	dm.LFLists = append(dm.LFLists, LFList{
		ListName: "N/A",
		Hash:     0,
		Content:  make(map[uint32]int),
	})
}

// loadLFListSingle 读一个禁卡表文件。文件不存在是常事（多数人没有 expansions），直接跳过。
func (dm *DeckManager) loadLFListSingle(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	var cur *LFList
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "!") {
			dm.LFLists = append(dm.LFLists, LFList{
				ListName: strings.TrimSpace(line[1:]),
				Hash:     lfListSeed,
				Content:  make(map[uint32]int),
			})
			cur = &dm.LFLists[len(dm.LFLists)-1]
			continue
		}
		if cur == nil {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		code, err := strconv.ParseUint(fields[0], 10, 32)
		if err != nil {
			continue
		}
		limit, err := strconv.Atoi(fields[1])
		// 限制数只能是 0/1/2（禁止/限制/准限制），别的值是脏行
		if err != nil || limit < 0 || limit > 2 {
			continue
		}
		cur.Content[uint32(code)] = limit
		cur.Hash = lfListHashStep(cur.Hash, uint32(code), limit)
	}
}

// lfListSeed 是禁卡表 hash 的种子（C++ deck_manager.cpp 里的 0x7dfcee6a）。
const lfListSeed uint32 = 0x7dfcee6a

// lfListHashStep 把一条禁限并进 hash。两处循环移位的位数与 C++ 完全一致 ——
// 差一位算出来就是另一个表，服务端会认为你用的禁卡表不对。
func lfListHashStep(h, code uint32, count int) uint32 {
	c := uint(count)
	return h ^ ((code << 18) | (code >> 14)) ^ ((code << (27 + c)) | (code >> (5 - c)))
}

func (dm *DeckManager) GetLFListName(lfhash uint32) string {
	for _, l := range dm.LFLists {
		if l.Hash == lfhash {
			return l.ListName
		}
	}
	return ""
}

func (dm *DeckManager) LoadDeck(path string) (*Deck, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	deck := &Deck{}
	section := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			if line == "!side" {
				section = "side"
			}
			continue
		}
		code, err := strconv.ParseUint(line, 10, 32)
		if err != nil {
			continue
		}
		switch section {
		case "side":
			deck.Side = append(deck.Side, uint32(code))
		default:
			if len(deck.Main) < DECK_MAX_SIZE {
				deck.Main = append(deck.Main, uint32(code))
			}
		}
	}
	return deck, scanner.Err()
}

func (dm *DeckManager) SaveDeck(deck *Deck, path string) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		os.MkdirAll(dir, 0755)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintln(f, "#created by ...")
	fmt.Fprintln(f, "#main")
	for _, c := range deck.Main {
		fmt.Fprintln(f, c)
	}
	fmt.Fprintln(f, "#extra")
	for _, c := range deck.Extra {
		fmt.Fprintln(f, c)
	}
	fmt.Fprintln(f, "!side")
	for _, c := range deck.Side {
		fmt.Fprintln(f, c)
	}
	return nil
}

// 卡组文件的枚举与定位。
//
// 此前客户端只能用硬编码的 deck/default.ydk：玩家没法选自己的卡组，
// 而联机时「准备」要把当前卡组发给服务器，等于只能拿同一副牌打。
//
// 目录规则照搬 C++（deck_manager.cpp 的 GetCategoryPath）：
// 默认分类就是 ./deck，具名分类是 ./deck/<分类名>，里面的 .ydk 就是卡组。

const deckRoot = "deck"

// DeckCategories 列出所有卡组分类。第一项固定是默认分类（空名，即 ./deck 本身），
// 其后是 ./deck 下的每个子目录。
func (dm *DeckManager) DeckCategories() []string {
	cats := []string{""}
	entries, err := os.ReadDir(deckRoot)
	if err != nil {
		return cats
	}
	for _, e := range entries {
		if e.IsDir() {
			cats = append(cats, e.Name())
		}
	}
	sort.Strings(cats[1:])
	return cats
}

// DeckNames 列出某个分类下的卡组名（不含 .ydk 后缀）。
func (dm *DeckManager) DeckNames(category string) []string {
	entries, err := os.ReadDir(CategoryPath(category))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		// 后缀比较要不分大小写：Windows 上 .YDK 一样是卡组
		if !strings.EqualFold(filepath.Ext(e.Name()), ".ydk") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
	}
	sort.Strings(names)
	return names
}

// CategoryPath 返回分类目录。空分类名表示默认的 ./deck。
func CategoryPath(category string) string {
	if category == "" {
		return deckRoot
	}
	return filepath.Join(deckRoot, category)
}

// DeckPath 返回某个卡组的文件路径。
func DeckPath(category, name string) string {
	return filepath.Join(CategoryPath(category), name+".ydk")
}

// LoadCurrentDeck 载入指定卡组并设为当前卡组，同时把选择记进配置
// （下次启动直接用，对应 C++ 的 lastcategory / lastdeck）。
func (dm *DeckManager) LoadCurrentDeck(category, name string) error {
	d, err := dm.LoadDeck(DeckPath(category, name))
	if err != nil {
		return err
	}
	dm.CurrentDeck = *d
	MainGame.Config.LastCategory = category
	MainGame.Config.LastDeck = name
	return nil
}
