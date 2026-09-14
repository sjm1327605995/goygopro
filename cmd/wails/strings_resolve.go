package main

// ResolveDesc 把引擎弹窗里的 desc 字符串 id 解码为真实文本，对应原版
// data_manager.cpp GetDesc（data_manager.cpp:262）：
//
//	strCode < 4000<<4(=64000)              → 系统字符串（strings.conf），仓库无此文件，返回空文本
//	code   = (strCode >> 4) & 0x0fffffff   → 卡号
//	offset = strCode & 0xf                 → 0=卡牌描述文本，1-15=texts 表 str1-str15（aux.Stringid）
//
// 卡片脚本几乎全用 aux.Stringid(code, n)（卡牌字符串），所以 cdb 路径可以
// 覆盖 select_yesno / select_option / select_effectyn 等绝大多数弹窗文本。
func (a *App) ResolveDesc(descID uint32) map[string]interface{} {
	res := map[string]interface{}{"success": false, "text": "", "code": uint32(0)}
	if descID < 4000<<4 {
		// 系统字符串：没有 strings.conf，前端自行兜底。
		res["success"] = true
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
