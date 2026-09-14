package duel

import (
	"sync"

	"github.com/sjm1327605995/goygopro/ocgcore"
)

var (
	serverDataMu    sync.Mutex
	serverDataReady bool
)

// InitServerData initializes the card database, banlist, and ocgcore API.
// This must be called before StartDuelServer. It is safe to call multiple
// times; the actual initialization only happens once, and only a successful
// one is remembered — a failed init can be retried by calling again.
func InitServerData(dbPath, scriptDir, rootPath string) error {
	serverDataMu.Lock()
	defer serverDataMu.Unlock()
	if serverDataReady {
		return nil
	}
	if err := DefaultDataManager.LoadDB(dbPath); err != nil {
		return err
	}
	DeckManger.LoadLFList()
	if err := ocgcore.Init(
		ocgcore.WithRootPath(rootPath),
		ocgcore.WithScriptDirectory(scriptDir),
		ocgcore.WithCardReader(func(cardId uint32) *ocgcore.CardData {
			return DefaultDataManager.GetData(cardId)
		}),
	); err != nil {
		return err
	}
	serverDataReady = true
	return nil
}
