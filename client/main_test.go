package client

import (
	"os"
	"testing"
)

// 界面文本来自仓库根目录的 strings.conf，测试里也要加载 —— 否则 Victory/System
// 全都回显编号，凡是断言文案的用例都会以一种很难看懂的方式失败。
//
// 注意别 Chdir：这个包里有测试自己切工作目录（卡组枚举、录像），
// 这里只按相对路径读一次。
func TestMain(m *testing.M) {
	if err := Strings.LoadStrings("../strings.conf"); err != nil {
		// 缺文件不算致命：那些断言具体文案的用例会自己 skip 或失败并说明原因
		os.Stderr.WriteString("测试环境没有 strings.conf: " + err.Error() + "\n")
	}
	os.Exit(m.Run())
}
