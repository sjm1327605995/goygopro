package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sjm1327605995/goygopro/core/duel"
)

// 禁卡表（lflist.conf）此前根本载不进来：段落头被当成 `[名称]`，
// 而真实文件用的是 `!名称` —— 一个表都读不到，卡组的禁限校验形同虚设。
//
// hash 尤其要紧：它是服务端匹配禁卡表的键。客户端算错的话，房间里选的表跟服务端
// 认的对不上号。服务端那份解析（core/duel）本来就是对的，所以这里拿它当基准交叉验证。

func withRealLFList(t *testing.T) {
	t.Helper()
	src := filepath.Join("..", "lflist.conf")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("没有 lflist.conf: %v", err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lflist.conf"), data, 0644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}

func TestLoadLFListReadsRealFile(t *testing.T) {
	withRealLFList(t)
	dm := &DeckManager{}

	dm.LoadLFList()

	if len(dm.LFLists) == 0 {
		t.Fatal("一个禁卡表都没载入 —— 段落头认错了（真实文件用 !名称）")
	}
	first := dm.LFLists[0]
	if first.ListName == "" {
		t.Error("禁卡表没有名字")
	}
	if len(first.Content) == 0 {
		t.Errorf("禁卡表 %q 里一条禁限都没有", first.ListName)
	}
	if first.Hash == lfListSeed {
		t.Error("hash 还是种子值 —— 每条禁限都该并进 hash")
	}
	// 限制数只能是 0/1/2
	for code, limit := range first.Content {
		if limit < 0 || limit > 2 {
			t.Errorf("卡 %d 的限制数 = %d, 只应是 0/1/2", code, limit)
			break
		}
	}
}

// 与服务端逐个表比对：名字、条目数、hash 都要一致。
// hash 差一位就是另一个表，服务端会认为客户端用的禁卡表不对。
func TestLFListMatchesServerParser(t *testing.T) {
	withRealLFList(t)

	dm := &DeckManager{}
	dm.LoadLFList()

	// 服务端是包级单例；先清空免得被别的用例的载入结果干扰
	srv := duel.DeckManger
	srv.LFList = nil
	srv.LoadLFList()

	if len(dm.LFLists) != len(srv.LFList) {
		t.Fatalf("客户端载入 %d 个表，服务端 %d 个 —— 两边解析不一致",
			len(dm.LFLists), len(srv.LFList))
	}
	for i := range dm.LFLists {
		c, s := dm.LFLists[i], srv.LFList[i]
		if c.ListName != s.ListName {
			t.Errorf("第 %d 个表名字：客户端 %q，服务端 %q", i, c.ListName, s.ListName)
			continue
		}
		if len(c.Content) != len(s.Content) {
			t.Errorf("表 %q 条目数：客户端 %d，服务端 %d", c.ListName, len(c.Content), len(s.Content))
		}
		if c.Hash != s.Hash {
			t.Errorf("表 %q 的 hash：客户端 %#x，服务端 %#x —— 服务端会认为禁卡表不对",
				c.ListName, c.Hash, s.Hash)
		}
	}
}

// GetLFListName 要能按 hash 查回名字，这是房间里显示禁卡表用的。
func TestGetLFListNameByHash(t *testing.T) {
	withRealLFList(t)
	dm := &DeckManager{}
	dm.LoadLFList()
	if len(dm.LFLists) == 0 {
		t.Skip("没载入禁卡表")
	}

	want := dm.LFLists[0]
	if got := dm.GetLFListName(want.Hash); got != want.ListName {
		t.Errorf("按 hash %#x 查到 %q, want %q", want.Hash, got, want.ListName)
	}
}
