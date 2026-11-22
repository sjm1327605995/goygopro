package tenon

import (
	"strconv"

	"github.com/sjm1327605995/tenon/pkg/shadcn"
)

var hostCardRuleOptions = []shadcn.SelectOption{
	{Label: "OCG", Value: "0"}, {Label: "TCG", Value: "1"}, {Label: "简体中文", Value: "2"},
	{Label: "自定义卡片", Value: "3"}, {Label: "无独有卡", Value: "4"}, {Label: "所有卡片", Value: "5"},
}

var hostModeOptions = []shadcn.SelectOption{
	{Label: "单局模式", Value: "0"}, {Label: "比赛模式", Value: "1"}, {Label: "TAG", Value: "2"},
}

var hostDuelRuleOptions = []shadcn.SelectOption{
	{Label: "大师规则（2020）", Value: "5"}, {Label: "新大师规则（2017）", Value: "4"},
	{Label: "大师规则３", Value: "3"}, {Label: "大师规则２", Value: "2"}, {Label: "大师规则", Value: "1"},
}

var filterKindOptions = []shadcn.SelectOption{
	{Label: "（无）", Value: "0"}, {Label: "怪兽", Value: "1"}, {Label: "魔法", Value: "2"}, {Label: "陷阱", Value: "4"},
}

var filterSubtypeOptions = []shadcn.SelectOption{
	{Label: "（无）", Value: "0"}, {Label: "通常", Value: "16"}, {Label: "效果", Value: "32"},
	{Label: "融合", Value: "64"}, {Label: "仪式", Value: "128"}, {Label: "同调", Value: "8192"},
	{Label: "灵摆", Value: "16777216"}, {Label: "超量", Value: "8388608"}, {Label: "连接", Value: "67108864"},
}

var filterAttributeOptions = []shadcn.SelectOption{
	{Label: "（无）", Value: "0"}, {Label: "地", Value: "1"}, {Label: "水", Value: "2"},
	{Label: "炎", Value: "4"}, {Label: "风", Value: "8"}, {Label: "光", Value: "16"},
	{Label: "暗", Value: "32"}, {Label: "神", Value: "64"},
}

var filterRaceOptions = []shadcn.SelectOption{
	{Label: "（无）", Value: "0"}, {Label: "战士", Value: "1"}, {Label: "魔法师", Value: "2"},
	{Label: "天使", Value: "4"}, {Label: "恶魔", Value: "8"}, {Label: "不死", Value: "16"},
	{Label: "机械", Value: "32"}, {Label: "水", Value: "64"}, {Label: "炎", Value: "128"},
	{Label: "岩石", Value: "256"}, {Label: "鸟兽", Value: "512"}, {Label: "植物", Value: "1024"},
	{Label: "昆虫", Value: "2048"}, {Label: "雷", Value: "4096"}, {Label: "龙", Value: "8192"},
	{Label: "兽", Value: "16384"}, {Label: "兽战士", Value: "32768"}, {Label: "恐龙", Value: "65536"},
	{Label: "鱼", Value: "131072"}, {Label: "海龙", Value: "262144"}, {Label: "爬虫类", Value: "524288"},
	{Label: "念动力", Value: "1048576"}, {Label: "幻龙", Value: "8388608"}, {Label: "电子界", Value: "16777216"},
}

func optionUint(values ...string) uint32 {
	for _, value := range values {
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err == nil {
			return uint32(parsed)
		}
	}
	return 0
}
