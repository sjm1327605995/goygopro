package scenes

import (
	"os"
	"testing"

	"github.com/sjm1327605995/goygopro/client"
)

// TestMain 把工作目录切到仓库根并初始化 ImageManager。
//
// 两件事都是必需的：贴图路径（textures/…）是相对仓库根的；而 ImageManager 没初始化时
// 连 TUnknown 占位图都没有，GetTexture 会返回 nil，界面上所有卡图都是空的 ——
// 曾因此误以为悬停预览的接线断了，其实只是测试环境没备好。
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	client.MainGame.Initialize() // 顺带把 strings.conf 读进来，界面文案才是真的
	os.Exit(m.Run())
}
