package main

import "github.com/sjm1327605995/goygopro/core/data"

// ResolveDesc 把引擎弹窗里的 desc 字符串 id 解码为真实文本，对应原版
// data_manager.cpp GetDesc（data_manager.cpp:262）：
//
//	strCode < MIN_CARD_ID<<4(=2048)                → 系统字符串（strings.conf），
//	                                                由下方内嵌的常用子集覆盖
//	code   = (strCode >> 4) & 0x0fffffff            → 卡号
//	offset = strCode & 0xf                          → 0=卡牌描述文本，1-15=texts 表 str1-str15（aux.Stringid）
//
// MIN_CARD_ID = (MAX_STRING_ID+1)>>4 = 128（与 core/data/data_manager.go 同源），
// 阈值即 2048。卡片脚本几乎全用 aux.Stringid(code, n)（卡牌字符串），所以 cdb 路径可以
// 覆盖 select_yesno / select_option / select_effectyn 等绝大多数弹窗文本。
// 系统字符串子集与前端 domain/sys_strings.ts 同源（仓库 strings.conf 摘录），
// 未收录的 id 返回空文本，前端自行兜底。
func (a *App) ResolveDesc(descID uint32) map[string]interface{} {
	if descID < data.MIN_CARD_ID<<4 {
		// 系统字符串：命中内嵌子集返回文案，否则空文本（调用方兜底）。
		return okResult("text", sysString(descID))
	}
	code := (descID >> 4) & 0x0fffffff
	offset := descID & 0xf
	var cd *CardInfo
	if a.cardDB != nil {
		cd = a.cardDB.GetCard(code)
	}
	if cd == nil {
		// 保持 success=false 契约，附 code 便于前端兜底。
		res := okResult("text", "", "code", code)
		res["success"] = false
		return res
	}
	text := ""
	if offset == 0 {
		text = cd.Desc
	} else if int(offset) <= len(cd.Strings) {
		text = cd.Strings[offset-1]
	}
	return okResult("text", text, "code", code)
}
