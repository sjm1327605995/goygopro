package main

// ResolveDesc 把引擎弹窗里的 desc 字符串 id 解码为真实文本，对应原版
// data_manager.cpp GetDesc（data_manager.cpp:262）：
//
//	strCode < 4000<<4(=64000)              → 系统字符串（strings.conf），
//	                                         由下方内嵌的常用子集覆盖
//	code   = (strCode >> 4) & 0x0fffffff   → 卡号
//	offset = strCode & 0xf                 → 0=卡牌描述文本，1-15=texts 表 str1-str15（aux.Stringid）
//
// 卡片脚本几乎全用 aux.Stringid(code, n)（卡牌字符串），所以 cdb 路径可以
// 覆盖 select_yesno / select_option / select_effectyn 等绝大多数弹窗文本。
// 系统字符串子集与前端 domain/sys_strings.ts 同源（仓库 strings.conf 摘录），
// 未收录的 id 返回空文本，前端自行兜底。
func (a *App) ResolveDesc(descID uint32) map[string]interface{} {
	res := map[string]interface{}{"success": false, "text": "", "code": uint32(0)}
	if descID < 4000<<4 {
		// 系统字符串：命中内嵌子集返回文案，否则空文本（调用方兜底）。
		res["success"] = true
		res["text"] = sysString(descID)
		return res
	}
	code := (descID >> 4) & 0x0fffffff
	offset := descID & 0xf
	res["code"] = code
	if a.cardDB == nil {
		return res
	}
	cd := a.cardDB.GetCard(code)
	if cd == nil {
		return res
	}
	if offset == 0 {
		res["text"] = cd.Desc
	} else if int(offset) <= len(cd.Strings) {
		res["text"] = cd.Strings[offset-1]
	}
	res["success"] = true
	return res
}
