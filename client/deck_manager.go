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

func (dm *DeckManager) LoadLFList() {
	f, err := os.Open("lflist.conf")
	if err != nil {
		return
	}
	defer f.Close()

	var current *LFList
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := line[1 : len(line)-1]
			current = &LFList{
				ListName: name,
				Content:  make(map[uint32]int),
			}
			dm.LFLists = append(dm.LFLists, *current)
			current = &dm.LFLists[len(dm.LFLists)-1]
			continue
		}
		if current == nil {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		code, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			continue
		}
		limit, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		current.Content[uint32(code)] = limit
	}
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
