package client

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// LFList corresponds to C++ struct LFList in deck_manager.h
type LFList struct {
	Hash    uint32
	ListName string
	Content map[uint32]int
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
