package client

// 「选卡凑数值」的可选性判定 —— MSG_SELECT_SUM（同调/超量素材、仪式献祭等）与
// MSG_SELECT_TRIBUTE（上级召唤的祭品）。翻译自 C++ gframe/client_field.cpp 的
// CheckSelectSum / CheckSelectTribute 及其一族递归辅助函数。
//
// 在此之前 Go 这边是 "Simplified: skip CheckSelectSum, allow any selection"：
// 只按张数卡 min/max，不看数值能不能凑出来。后果是玩家可以选出**凑不成的组合**、
// 界面还显示为合法，点确定才被服务器拒绝。
//
// 每张候选卡的 opParam 里塞了一个或两个「可用数值」（如怪兽等级、或等级/阶级二选一），
// 由 sumParams 解出。问题本身是子集和：给定一堆卡，问「还能不能凑出目标值」，
// 因此下面几个函数都是回溯搜索。牌堆规模只有十几张，指数级在这里是够用的。
//
// 与 C++ 的一处实现差异：C++ 用 std::set 存候选，这里用切片。set 会按指针排序、
// 顺序不定；切片保持服务器给的顺序，结果集合相同（都是「是否存在一个可行子集」），
// 但更可复现。

// ToggleSumPick 处理「凑数值」选卡框里的一次点击：选中/取消这张卡，然后重算
// 哪些卡还能选、当前是否已经可以提交。
//
// 必须每点一次都重算 —— 只在开局算一次的话，玩家选掉几张之后剩下的卡是否还可行
// 就没人管了，界面会继续让他点出凑不成的组合。
// curMsg 用来区分是凑数值（MSG_SELECT_SUM）还是选祭品（MSG_SELECT_TRIBUTE）。
func (cf *ClientField) ToggleSumPick(card *ClientCard, curMsg int16) {
	if card.IsSelected {
		// 强制选中的（前 MustSelectCount 张）不能取消
		idx := -1
		for i, c := range cf.SelectedCards {
			if c == card {
				idx = i
				break
			}
		}
		if idx < 0 || idx < cf.MustSelectCount {
			return
		}
		cf.SelectedCards = append(cf.SelectedCards[:idx], cf.SelectedCards[idx+1:]...)
	} else {
		if !card.IsSelectable {
			return
		}
		cf.SelectedCards = append(cf.SelectedCards, card)
	}

	if curMsg == msgSelectTribute {
		cf.SelectReady = cf.CheckSelectTribute()
	} else {
		cf.SelectReady = cf.CheckSelectSum()
	}
}

// 这两个值与 protocol/network 中的同名常量一致。放在这里是为了让 client 包的核心
// 选卡逻辑不依赖网络包 —— 它描述的是规则，不是线上格式。
const (
	msgSelectTribute = 20
	msgSelectSum     = 23
)

// sumParams 解出一张卡的两个候选数值。
// 对应 C++ get_sum_params：高位带 0x8000 标记时，整个 opParam（去掉符号位）是单个大数值，
// 否则低 16 位、高 16 位各是一个可选数值（op2 为 0 表示只有一个）。
func sumParams(opParam uint32) (op1, op2 int) {
	op1 = int(opParam & 0xffff)
	op2 = int((opParam >> 16) & 0xffff)
	if op2&0x8000 != 0 {
		op1 = int(opParam & 0x7fffffff)
		op2 = 0
	}
	return
}

// minOf 返回一张卡能贡献的最小数值（两个候选里较小的那个）。
func minParam(op1, op2 int) int {
	if op2 > 0 && op1 > op2 {
		return op2
	}
	return op1
}

func maxParam(op1, op2 int) int {
	if op2 > op1 {
		return op2
	}
	return op1
}

// removeCard 返回去掉 target 后的新切片（不改原切片）。
func removeCard(list []*ClientCard, target *ClientCard) []*ClientCard {
	out := make([]*ClientCard, 0, len(list))
	for _, c := range list {
		if c != target {
			out = append(out, c)
		}
	}
	return out
}

// CheckSelectSum 重算「凑数值」时的可选状态：谁还能点、当前是否已经可以确定。
// 返回值表示当前已选组合是否合法（可以提交）。
//
// 对应 C++ ClientField::CheckSelectSum。
func (cf *ClientField) CheckSelectSum() bool {
	selable := make([]*ClientCard, 0, len(cf.SelectSumAll))
	for _, sc := range cf.SelectSumAll {
		sc.IsSelectable = false
		sc.IsSelected = false
		selable = append(selable, sc)
	}

	cf.SelectCurValL = 0
	cf.SelectCurValH = 0
	for i, sc := range cf.SelectedCards {
		// 前 MustSelectCount 张是强制选中的，不能取消
		sc.IsSelectable = i >= cf.MustSelectCount
		sc.IsSelected = true
		selable = removeCard(selable, sc)

		op1, op2 := sumParams(sc.OpParam)
		cf.SelectCurValL += minParam(op1, op2)
		cf.SelectCurValH += maxParam(op1, op2)
	}

	cf.SelectSumCards = make(map[*ClientCard]struct{})

	if cf.SelectMode == 0 {
		return cf.checkSumEqual(selable)
	}
	return cf.checkSumGreater(selable)
}

// checkSumEqual 是 select_mode == 0：数值之和必须**正好等于**目标。
func (cf *ClientField) checkSumEqual(selable []*ClientCard) bool {
	ret := cf.checkSelSumS(selable, 0, cf.SelectSumVal)
	cf.rebuildSelectable()
	return ret
}

// checkSumGreater 是 select_mode != 0：和只需**达到**目标，但不能有多余的一张
// （去掉任意一张就不够了）—— 即「刚好够」。
func (cf *ClientField) checkSumGreater(selable []*ClientCard) bool {
	mm, mx := -1, -1
	sumc, max := 0, 0
	for _, sc := range cf.SelectedCards {
		op1, op2 := sumParams(sc.OpParam)
		opmin, opmax := minParam(op1, op2), maxParam(op1, op2)
		if mm == -1 || opmin < mm {
			mm = opmin
		}
		if mx == -1 || opmax < mx {
			mx = opmax
		}
		sumc += opmin
		max += opmax
	}

	if cf.SelectSumVal <= sumc {
		return true
	}
	ret := cf.SelectSumVal <= max && cf.SelectSumVal > max-mx

	for _, sc := range selable {
		op1, op2 := sumParams(sc.OpParam)
		for _, m := range []int{op1, op2} {
			if m == op2 && op2 == 0 {
				continue // 只有一个候选值
			}
			sums := sumc + m
			ms := mm
			if ms == -1 || m < ms {
				ms = m
			}
			if sums >= cf.SelectSumVal {
				// 加上这张就够了；但必须「刚好够」——去掉最小的一张就不够
				if sums-ms < cf.SelectSumVal {
					cf.SelectSumCards[sc] = struct{}{}
				}
				continue
			}
			// 还不够：剩下的卡里得存在能补齐差额的组合
			left := removeCard(selable, sc)
			if cf.checkMin(left, 0, cf.SelectSumVal-sums, cf.SelectSumVal-sums+ms-1) {
				cf.SelectSumCards[sc] = struct{}{}
			}
		}
	}

	cf.rebuildSelectable()
	return ret
}

// rebuildSelectable 把算出来的可选集合写回 SelectableCards，并标记 IsSelectable。
// 已选中的卡也留在列表里 —— 它们要能被点掉。
func (cf *ClientField) rebuildSelectable() {
	cf.SelectableCards = nil
	for _, sc := range cf.SelectSumAll {
		if _, ok := cf.SelectSumCards[sc]; !ok {
			continue
		}
		sc.IsSelectable = true
		cf.SelectableCards = append(cf.SelectableCards, sc)
	}
	cf.SelectableCards = append(cf.SelectableCards, cf.SelectedCards...)
}

// checkMin 问：left 里能否选出若干张，其最小数值之和落在 [min, max] 内。
// 对应 C++ check_min。
func (cf *ClientField) checkMin(left []*ClientCard, index, min, max int) bool {
	if index >= len(left) {
		return false
	}
	op1, op2 := sumParams(left[index].OpParam)
	m := minParam(op1, op2)
	if m >= min && m <= max {
		return true
	}
	return (min > m && cf.checkMin(left, index+1, min-m, max-m)) ||
		cf.checkMin(left, index+1, min, max)
}

// checkSelSumS 先把已选中的卡逐一扣掉（每张可能有两个候选值，故分叉），
// 扣完还差 acc 时再看剩下的卡能否补齐。对应 C++ check_sel_sum_s。
func (cf *ClientField) checkSelSumS(left []*ClientCard, index, acc int) bool {
	if acc < 0 {
		return false
	}
	if index == len(cf.SelectedCards) {
		if acc == 0 {
			count := len(cf.SelectedCards) - cf.MustSelectCount
			return count >= cf.SelectMin && count <= cf.SelectMax
		}
		// 还差 acc：把能补齐的候选卡记进 SelectSumCards
		cf.checkSelSumT(left, acc)
		return false
	}
	l1, l2 := sumParams(cf.SelectedCards[index].OpParam)
	res := cf.checkSelSumS(left, index+1, acc-l1)
	if l2 > 0 {
		res = cf.checkSelSumS(left, index+1, acc-l2) || res
	}
	return res
}

// checkSelSumT 逐张试探：把这张也选上之后，剩下的卡能否把 acc 补齐。
// 能则它是可选的。对应 C++ check_sel_sum_t。
func (cf *ClientField) checkSelSumT(left []*ClientCard, acc int) {
	count := len(cf.SelectedCards) + 1 - cf.MustSelectCount
	for _, sc := range left {
		if _, ok := cf.SelectSumCards[sc]; ok {
			continue
		}
		testlist := removeCard(left, sc)
		l1, l2 := sumParams(sc.OpParam)
		if cf.checkSum(testlist, 0, acc-l1, count) ||
			(l2 > 0 && cf.checkSum(testlist, 0, acc-l2, count)) {
			cf.SelectSumCards[sc] = struct{}{}
		}
	}
}

// checkSum 是子集和的回溯：从 index 起能否凑出 acc，且总张数落在 [SelectMin, SelectMax]。
// 对应 C++ check_sum。
func (cf *ClientField) checkSum(list []*ClientCard, index, acc, count int) bool {
	if acc == 0 {
		return count >= cf.SelectMin && count <= cf.SelectMax
	}
	if acc < 0 || index >= len(list) {
		return false
	}
	l1, l2 := sumParams(list[index].OpParam)
	if (l1 == acc || (l2 > 0 && l2 == acc)) && count+1 >= cf.SelectMin && count+1 <= cf.SelectMax {
		return true
	}
	return (acc > l1 && cf.checkSum(list, index+1, acc-l1, count+1)) ||
		(l2 > 0 && acc > l2 && cf.checkSum(list, index+1, acc-l2, count+1)) ||
		cf.checkSum(list, index+1, acc, count)
}

// CheckSelectTribute 是祭品选择版：这里的目标不是「和等于某值」，而是
// 「祭品提供的总数落在 [SelectMin, SelectMax]」。对应 C++ CheckSelectTribute。
func (cf *ClientField) CheckSelectTribute() bool {
	selable := make([]*ClientCard, 0, len(cf.SelectSumAll))
	for _, sc := range cf.SelectSumAll {
		sc.IsSelectable = false
		sc.IsSelected = false
		selable = append(selable, sc)
	}
	for _, sc := range cf.SelectedCards {
		sc.IsSelectable = true
		sc.IsSelected = true
		selable = removeCard(selable, sc)
	}

	cf.SelectSumCards = make(map[*ClientCard]struct{})
	ret := cf.checkSelSumTribS(selable, 0, 0)

	// 与 CheckSelectSum 不同：C++ 这里没有把 selected_cards 追加进 selectable_cards
	cf.SelectableCards = nil
	for _, sc := range cf.SelectSumAll {
		if _, ok := cf.SelectSumCards[sc]; !ok {
			continue
		}
		sc.IsSelectable = true
		cf.SelectableCards = append(cf.SelectableCards, sc)
	}
	return ret
}

func (cf *ClientField) checkSelSumTribS(left []*ClientCard, index, acc int) bool {
	if acc > cf.SelectMax {
		return false
	}
	if index == len(cf.SelectedCards) {
		cf.checkSelSumTribT(left, acc)
		return acc >= cf.SelectMin && acc <= cf.SelectMax
	}
	l1, l2 := sumParams(cf.SelectedCards[index].OpParam)
	res := cf.checkSelSumTribS(left, index+1, acc+l1)
	if l2 > 0 {
		res = cf.checkSelSumTribS(left, index+1, acc+l2) || res
	}
	return res
}

func (cf *ClientField) checkSelSumTribT(left []*ClientCard, acc int) {
	for _, sc := range left {
		if _, ok := cf.SelectSumCards[sc]; ok {
			continue
		}
		testlist := removeCard(left, sc)
		l1, l2 := sumParams(sc.OpParam)
		if cf.checkSumTrib(testlist, 0, acc+l1) ||
			(l2 > 0 && cf.checkSumTrib(testlist, 0, acc+l2)) {
			cf.SelectSumCards[sc] = struct{}{}
		}
	}
}

func (cf *ClientField) checkSumTrib(list []*ClientCard, index, acc int) bool {
	if acc >= cf.SelectMin && acc <= cf.SelectMax {
		return true
	}
	if acc > cf.SelectMax || index >= len(list) {
		return false
	}
	l1, l2 := sumParams(list[index].OpParam)
	if (acc+l1 >= cf.SelectMin && acc+l1 <= cf.SelectMax) ||
		(l2 > 0 && acc+l2 >= cf.SelectMin && acc+l2 <= cf.SelectMax) {
		return true
	}
	return cf.checkSumTrib(list, index+1, acc+l1) ||
		(l2 > 0 && cf.checkSumTrib(list, index+1, acc+l2)) ||
		cf.checkSumTrib(list, index+1, acc)
}
