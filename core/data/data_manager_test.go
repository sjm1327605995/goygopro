package data

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sjm1327605995/goygopro/ocgcore"
)

// TestDuelCodePrecedence 守住原版 get_duel_code() 的取值优先级
//（data_manager.h:60-62）：rule_code ?: alias ?: code。
func TestDuelCodePrecedence(t *testing.T) {
	plain := &CardDataC{}
	plain.Code = 100
	if got := plain.DuelCode(); got != 100 {
		t.Fatalf("无 rule_code/alias 应取 code：got %d", got)
	}
	alt := &CardDataC{}
	alt.Code, alt.Alias = 100, 200
	if got := alt.DuelCode(); got != 200 {
		t.Fatalf("异画卡应取 alias：got %d", got)
	}
	rule := &CardDataC{}
	rule.Code, rule.Alias, rule.RuleCode = 100, 200, 300
	if got := rule.DuelCode(); got != 300 {
		t.Fatalf("rule_code 优先于 alias：got %d", got)
	}
}

// TestLoadDBRuleCodeInvariants 对照 data_manager.cpp:61-68/86-92 的 rule_code
// 推导语义：迁移后 RuleCode 非零的卡 Alias 必须已清零；仍带 Alias 的卡只能
// 是 token 或异画（is_alternative 范围内）；5405695 特例生效。
func TestLoadDBRuleCodeInvariants(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(root, "cards.cdb")
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("cards.cdb not present: %v", err)
	}
	if err := DefaultDataManager.LoadDB(dbPath); err != nil {
		t.Fatal(err)
	}

	cards := 0
	for code, cd := range datas {
		cards++
		if cd.Alias != 0 && cd.Type&ocgcore.TYPE_TOKEN == 0 && !isAlternativeCode(cd.Code, cd.Alias) {
			t.Fatalf("卡 %d：残留 alias=%d 但非 token 非异画（rule_code 迁移漏网）", code, cd.Alias)
		}
		if cd.RuleCode != 0 && cd.Alias != 0 {
			// 第二趟继承（data_manager.cpp:89-94）：异画卡保留 alias 并继承
			// alias 目标的 rule_code。
			target, ok := datas[cd.Alias]
			if !ok {
				t.Fatalf("卡 %d：alias 目标 %d 不在卡库中", code, cd.Alias)
			}
			if target.RuleCode != cd.RuleCode {
				t.Fatalf("卡 %d：rule_code=%d 与 alias 目标 %d 的 rule_code=%d 不一致（继承语义错误）",
					code, cd.RuleCode, cd.Alias, target.RuleCode)
			}
		}
	}
	if cards == 0 {
		t.Fatal("卡库为空")
	}

	bls, ok := datas[ocgcore.CARD_BLACK_LUSTER_SOLDIER2]
	if ok && bls.RuleCode == 0 {
		t.Fatal("5405695 特例未生效：rule_code 为空")
	}
}

// TestLoadDBExtraSetCode 对照 data_manager.cpp:24-27/96-104：两张硬编码卡
// 的系列码被无条件覆盖为补丁值。
func TestLoadDBExtraSetCode(t *testing.T) {
	if len(datas) == 0 {
		root, err := filepath.Abs("../..")
		if err != nil {
			t.Fatal(err)
		}
		dbPath := filepath.Join(root, "cards.cdb")
		if _, err := os.Stat(dbPath); err != nil {
			t.Skipf("cards.cdb not present: %v", err)
		}
		if err := DefaultDataManager.LoadDB(dbPath); err != nil {
			t.Fatal(err)
		}
	}
	want := []uint16{0x8f, 0x54, 0x59, 0x82, 0x13a}
	for _, code := range []uint32{8512558, 55088578} {
		cd, ok := datas[code]
		if !ok {
			continue // 卡库不含该卡则无从覆盖
		}
		for i, v := range want {
			if cd.Setcode[i] != v {
				t.Fatalf("卡 %d 的 extra_setcode 覆盖缺失：Setcode[%d]=%04x want %04x", code, i, cd.Setcode[i], v)
			}
		}
	}
}
