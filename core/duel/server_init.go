package duel

import (
	"sync"

	"github.com/sjm1327605995/goygopro/core/data"
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
//
// 组装点：本函数是 core/data（卡库/禁限表/卡组校验，data_manager.go、
// deck_manager.go）与 ocgcore（引擎 API）的组装处，留在 core/duel 是因为
// 服务端各模式（single/tag）与 cmd 层都经 duel 包拿到初始化后的单例，
// 提到 cmd 层会迫使 core/duel 反向依赖具体可执行入口，改动大于收益。
func InitServerData(dbPath, scriptDir, rootPath string) error {
	serverDataMu.Lock()
	defer serverDataMu.Unlock()
	if serverDataReady {
		return nil
	}
	if err := data.DefaultDataManager.LoadDB(dbPath); err != nil {
		return err
	}
	data.DeckManager.LoadLFList()
	if err := ocgcore.Init(
		ocgcore.WithRootPath(rootPath),
		ocgcore.WithScriptDirectory(scriptDir),
		ocgcore.WithCardReader(func(cardId uint32) *ocgcore.CardData {
			return data.DefaultDataManager.GetData(cardId)
		}),
	); err != nil {
		return err
	}
	serverDataReady = true
	return nil
}
