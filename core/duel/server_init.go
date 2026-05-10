package duel

import (
	"sync"

	"github.com/sjm1327605995/goygopro/ocgcore"
)

var serverDataOnce sync.Once

// InitServerData initializes the card database, banlist, and ocgcore API.
// This must be called before StartDuelServer. It is safe to call multiple
// times; the actual initialization only happens once per process.
func InitServerData(dbPath, scriptDir, rootPath string) error {
	var initErr error
	serverDataOnce.Do(func() {
		if err := DefaultDataManager.LoadDB(dbPath); err != nil {
			initErr = err
			return
		}
		DeckManger.LoadLFList()
		if err := ocgcore.Init(
			ocgcore.WithRootPath(rootPath),
			ocgcore.WithScriptDirectory(scriptDir),
			ocgcore.WithCardReader(func(cardId uint32) *ocgcore.CardData {
				return DefaultDataManager.GetData(cardId)
			}),
		); err != nil {
			initErr = err
			return
		}
	})
	return initErr
}
