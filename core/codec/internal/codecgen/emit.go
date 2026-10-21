package main

import (
	"bytes"
	"fmt"
	"go/types"
)

// ---------------------------------------------------------------------------
// 生成范围判定
// ---------------------------------------------------------------------------

// candidate 报告结构体是否需要生成编解码方法：带 codec tag/标记字段，
// 或「全部字段导出且字段类型均为定宽整型/bool/数组/slice/指针/嵌套结构体」
// 的候选布局（嵌套结构体自身是否合法由它的生成与校验负责）。
func (g *gen) candidate(named *types.Named) bool {
	st := named.Underlying().(*types.Struct)
	hasSignal := false
	allExported := true
	basicOK := true
	for i := 0; i < st.NumFields(); i++ {
		v := st.Field(i)
		tag := st.Tag(i)
		for _, key := range []string{"count", "if", "bit", "greedy", "skip"} {
			if _, ok := lookupTag(tag, key); ok {
				hasSignal = true
			}
		}
		if isRestType(v.Type()) {
			hasSignal = true
		}
		if _, ok := isWireLen(v.Type()); ok {
			hasSignal = true
		}
		if _, ok := isCountMarker(v.Type()); ok {
			hasSignal = true
		}
		if !v.Exported() {
			allExported = false
		}
		if !g.codecBasic(v.Type(), map[types.Type]bool{}) {
			basicOK = false
		}
	}
	return hasSignal || (allExported && basicOK)
}

func (g *gen) codecBasic(t types.Type, seen map[types.Type]bool) bool {
	if _, _, _, ok := basicInfo(t); ok {
		return true
	}
	if _, ok := isWireLen(t); ok {
		return true
	}
	if _, ok := isCountMarker(t); ok {
		return true
	}
	if seen[t] {
		return true // 递归类型按候选处理（实际目标包无此情形）
	}
	seen[t] = true
	switch u := t.Underlying().(type) {
	case *types.Array:
		return g.codecBasic(u.Elem(), seen)
	case *types.Slice:
		return g.codecBasic(u.Elem(), seen)
	case *types.Pointer:
		return g.codecBasic(u.Elem(), seen)
	case *types.Struct:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// 布局条目（对应旧 buildPlan 的 fieldPlan 序列）
// ---------------------------------------------------------------------------

type itemKind int

const (
	kindScalar      itemKind = iota // 定宽整型/布尔
	kindArrayScalar                 // 定长标量/布尔数组
	kindArrayStruct                 // 定长结构体数组
	kindNested                      // 嵌套结构体值字段
	kindSkip
	kindGreedy
	kindWireLenBlob   // WireLen + []byte
	kindWireLenStruct // WireLen + 定长结构体
	kindWireLenSlice  // WireLen + 定宽元素 slice
	kindCountMarker   // Count 标记前缀（绑定目标另行出现）
	kindCountList     // count 列表（tag 或 Count 标记绑定）
	kindPtrScalar     // 指针字段：标量
	kindPtrStruct     // 指针字段：结构体
	kindPtrSlice      // 指针字段：带 count 的 slice
	kindPtrArray      // 指针字段：定长数组
)

type countRef struct {
	fieldName string // 计数字段名（消息用）
	viaPtr    bool   // 计数字段经指针（*uint32 条件字段）
	width     int    // 计数字段线宽
	signed    bool
	marker    bool // Count 标记类型（uint 无符号）
}

type condRef struct {
	fieldName string
	viaPtr    bool
	signed    bool
	mask      uint64
	maskAll   bool // 省略 &掩码
}

type item struct {
	kind itemKind
	f    *fieldMeta // 主字段

	width  int  // 标量线宽
	signed bool // 标量符号性
	isBool bool

	elemCount      int // 数组长度
	elemWidth      int // 数组/列表元素线宽
	elemSigned     bool
	elemIsBool     bool
	elemIsByte     bool // 元素 uint8（可整块 copy）
	elemIsStruct   bool
	elemTypeName   string     // 元素类型书写
	elemGoType     types.Type // 元素原始类型
	elemFixed      int        // 结构体元素线宽
	targetTypeName string

	wl          *fieldMeta // WireLen 标记字段
	wlWidth     int
	target      *fieldMeta // WireLen / Count 绑定目标字段
	targetFixed int        // WireLen 结构体目标线宽

	skipN int
	ref   countRef // count 列表的计数引用
	cond  *condRef // 指针字段谓词（nil = 无条件）
}

// buildItems 把校验过的结构体元数据编译成布局条目序列。
func (g *gen) buildItems(meta *structMeta) ([]*item, string, *fieldMeta) {
	g.curMeta = meta
	var items []*item
	for i := 0; i < len(meta.fields); i++ {
		f := meta.fields[i]
		switch {
		case f.isWL:
			next := meta.fields[i+1]
			it := &item{kind: kindWireLenBlob, wl: f, wlWidth: f.wlWidth, target: next}
			switch u := next.typ.Underlying().(type) {
			case *types.Slice:
				if !isUint8(u.Elem()) {
					it.kind = kindWireLenSlice
					es, msg := g.fixedSize(u.Elem())
					if msg != "" {
						return nil, msg, next
					}
					it.elemWidth = es
					_, it.elemIsStruct = u.Elem().Underlying().(*types.Struct)
					it.elemTypeName = g.typeRef(u.Elem())
					it.elemGoType = u.Elem()
				}
			case *types.Struct:
				it.kind = kindWireLenStruct
				sz, _ := g.fixedSize(next.typ)
				it.targetFixed = sz
				it.targetTypeName = g.typeRef(next.typ)
			}
			items = append(items, it)
			i++ // 目标字段随 WireLen 一并处理
		case f.isCM:
			items = append(items, &item{kind: kindCountMarker, f: f, width: f.cmWidth})
		case f.hasSkip:
			items = append(items, &item{kind: kindSkip, f: f, skipN: f.skip})
		default:
			it, msg, bf := g.buildValueItem(meta, f)
			if msg != "" {
				return nil, msg, bf
			}
			items = append(items, it)
		}
	}
	return items, "", nil
}

func (g *gen) buildValueItem(meta *structMeta, f *fieldMeta) (*item, string, *fieldMeta) {
	it := &item{f: f}
	u := f.typ.Underlying()
	if w, signed, isBool, ok := basicInfo(f.typ); ok {
		it.kind = kindScalar
		it.width, it.signed, it.isBool = w, signed, isBool
		return it, "", nil
	}
	switch v := u.(type) {
	case *types.Struct:
		it.kind = kindNested
		it.targetTypeName = g.typeRef(f.typ)
		return it, "", nil
	case *types.Array:
		it.elemCount = int(v.Len())
		if w, signed, isBool, ok := basicInfo(v.Elem()); ok {
			it.kind = kindArrayScalar
			it.elemWidth, it.elemSigned, it.elemIsBool = w, signed, isBool
			it.elemIsByte = w == 1 && !signed && !isBool
			it.elemTypeName = g.typeRef(v.Elem())
			it.elemGoType = v.Elem()
			return it, "", nil
		}
		if _, ok := v.Elem().Underlying().(*types.Struct); ok {
			es, msg := g.fixedSize(v.Elem())
			if msg != "" {
				return nil, msg, f
			}
			it.kind = kindArrayStruct
			it.elemWidth = es
			it.elemIsStruct = true
			it.elemTypeName = g.typeRef(v.Elem())
			it.elemGoType = v.Elem()
			return it, "", nil
		}
		return nil, fmt.Sprintf("不支持的类型 %s", v.Elem().Underlying().String()), f
	case *types.Slice:
		if f.greedy || f.rest {
			it.kind = kindGreedy
			return it, "", nil
		}
		// count 列表（tag 或 Count 标记绑定）
		if f.countBound {
			marker := meta.fields[f.boundBy]
			it.ref = countRef{fieldName: marker.name, width: marker.cmWidth}
			it.kind = kindCountList
		} else {
			cm := meta.fields[meta.byName[f.count]]
			ref := countRef{fieldName: cm.name}
			ct := cm.typ
			if p, ok := ct.Underlying().(*types.Pointer); ok {
				ref.viaPtr = true
				ct = p.Elem()
			}
			w, signed, _, ok := basicInfo(ct)
			if !ok {
				return nil, fmt.Sprintf("count 目标字段 %q 不是整型（或整型指针）", f.count), f
			}
			ref.width, ref.signed = w, signed
			it.ref = ref
			it.kind = kindCountList
		}
		if w, signed, isBool, ok := basicInfo(v.Elem()); ok {
			it.elemWidth, it.elemSigned, it.elemIsBool = w, signed, isBool
			it.elemIsByte = w == 1 && !signed && !isBool
			it.elemTypeName = g.typeRef(v.Elem())
			it.elemGoType = v.Elem()
			return it, "", nil
		}
		if _, ok := v.Elem().Underlying().(*types.Struct); ok {
			es, msg := g.fixedSize(v.Elem())
			if msg != "" {
				return nil, msg, f
			}
			it.elemWidth = es
			it.elemIsStruct = true
			it.elemTypeName = g.typeRef(v.Elem())
			it.elemGoType = v.Elem()
			return it, "", nil
		}
		return nil, fmt.Sprintf("不支持的类型 %s", v.Elem().Underlying().String()), f
	case *types.Pointer:
		return g.buildPtrItem(meta, f, v)
	}
	return nil, fmt.Sprintf("不支持的类型 %s", u.String()), f
}

func (g *gen) buildPtrItem(meta *structMeta, f *fieldMeta, v *types.Pointer) (*item, string, *fieldMeta) {
	it := &item{f: f}
	switch {
	case f.hasBit:
		// bit:"N"：谓词是声明序向前最近的前导整型字段的第 N 位
		cm := meta.fields[f.bitPred]
		it.cond = &condRef{fieldName: cm.name, mask: uint64(1) << uint(f.bit)}
	case f.hasCond:
		cm := meta.fields[meta.byName[f.cond]]
		c := &condRef{fieldName: cm.name, mask: f.mask, maskAll: f.mask == ^uint64(0)}
		ct := cm.typ
		if p, ok := ct.Underlying().(*types.Pointer); ok {
			c.viaPtr = true
			ct = p.Elem()
		}
		_, c.signed, _, _ = basicInfo(ct)
		it.cond = c
	}
	elem := v.Elem()
	if sl, ok := elem.Underlying().(*types.Slice); ok && f.count != "" {
		it.kind = kindPtrSlice
		cm := meta.fields[meta.byName[f.count]]
		ref := countRef{fieldName: cm.name}
		ct := cm.typ
		if p, ok := ct.Underlying().(*types.Pointer); ok {
			ref.viaPtr = true
			ct = p.Elem()
		}
		w, signed, _, ok := basicInfo(ct)
		if !ok {
			return nil, fmt.Sprintf("count 目标字段 %q 不是整型（或整型指针）", f.count), f
		}
		ref.width, ref.signed = w, signed
		it.ref = ref
		if w, signed, isBool, ok := basicInfo(sl.Elem()); ok {
			it.elemWidth, it.elemSigned, it.elemIsBool = w, signed, isBool
			it.elemIsByte = w == 1 && !signed && !isBool
			it.elemTypeName = g.typeRef(sl.Elem())
			it.elemGoType = sl.Elem()
			return it, "", nil
		}
		if _, ok := sl.Elem().Underlying().(*types.Struct); ok {
			es, msg := g.fixedSize(sl.Elem())
			if msg != "" {
				return nil, msg, f
			}
			it.elemWidth = es
			it.elemIsStruct = true
			it.elemTypeName = g.typeRef(sl.Elem())
			it.elemGoType = sl.Elem()
			return it, "", nil
		}
		return nil, fmt.Sprintf("不支持的类型 %s", sl.Elem().Underlying().String()), f
	}
	if w, signed, isBool, ok := basicInfo(elem); ok {
		it.kind = kindPtrScalar
		it.width, it.signed, it.isBool = w, signed, isBool
		return it, "", nil
	}
	if _, ok := elem.Underlying().(*types.Struct); ok {
		it.kind = kindPtrStruct
		it.targetTypeName = g.typeRef(elem)
		return it, "", nil
	}
	if arr, ok := elem.Underlying().(*types.Array); ok {
		it.kind = kindPtrArray
		it.elemCount = int(arr.Len())
		if w, signed, isBool, ok := basicInfo(arr.Elem()); ok {
			it.elemWidth, it.elemSigned, it.elemIsBool = w, signed, isBool
			it.elemIsByte = w == 1 && !signed && !isBool
			it.elemTypeName = g.typeRef(arr.Elem())
			it.elemGoType = arr.Elem()
			return it, "", nil
		}
		return nil, fmt.Sprintf("不支持的类型 %s", arr.Elem().Underlying().String()), f
	}
	return nil, fmt.Sprintf("指针目标类型 %s 不支持", elem.Underlying().String()), f
}

// ---------------------------------------------------------------------------
// 代码发射
// ---------------------------------------------------------------------------

func (g *gen) emitStruct(buf *bytes.Buffer, name string, named *types.Named) bool {
	st := named.Underlying().(*types.Struct)
	meta, msg, bad := g.validate(name, st)
	if msg != "" {
		g.emitStub(buf, name, msg, bad)
		return false
	}
	items, msg, bad := g.buildItems(meta)
	if msg != "" {
		g.emitStub(buf, name, msg, bad)
		return false
	}

	fmt.Fprintf(buf, "func (m *%s) CodecGenPtr() {}\n\n", name)
	// CodecSize（指针接收者：原地回填 count/WireLen）
	fmt.Fprintf(buf, "func (m *%s) CodecSize() (int, error) {\n\tn := 0\n", name)
	g.emitBody(buf, name, items, bodySize)
	buf.WriteString("\treturn n, nil\n}\n\n")
	// CodecMarshalTo（指针接收者：读已回填的结构体）
	fmt.Fprintf(buf, "func (m *%s) CodecMarshalTo(dst []byte) int {\n\toff := 0\n", name)
	g.emitBody(buf, name, items, bodyMarshal)
	buf.WriteString("\treturn off\n}\n\n")
	// CodecSizeVal（值接收者：回填写到副本，供结构体值入参的 WireSize）
	fmt.Fprintf(buf, "func (m %s) CodecSizeVal() (int, error) {\n\tn := 0\n", name)
	g.emitBody(buf, name, items, bodySize)
	buf.WriteString("\treturn n, nil\n}\n\n")
	// CodecEncodeInto（值接收者：副本回填 + 容量检查 + 编码）
	fmt.Fprintf(buf, "func (m %s) CodecEncodeInto(dst []byte) (int, error) {\n\tn := 0\n", name)
	g.emitBody(buf, name, items, bodySize)
	fmt.Fprintf(buf, "\tif n > len(dst) {\n\t\treturn 0, %sCapacityErr(%q, n, len(dst))\n\t}\n", g.q(""), name)
	buf.WriteString("\toff := 0\n")
	g.emitBody(buf, name, items, bodyMarshal)
	buf.WriteString("\treturn off, nil\n}\n\n")
	// CodecUnmarshalFrom
	fmt.Fprintf(buf, "func (m *%s) CodecUnmarshalFrom(src []byte) (int, error) {\n", name)
	fmt.Fprintf(buf, "\tif m == nil {\n\t\treturn 0, %sFieldErr(%q, %q, \"\", 0, \"nil 结构体指针\")\n\t}\n",
		g.q(""), "unmarshal", name)
	buf.WriteString("\toff := 0\n")
	g.emitBody(buf, name, items, bodyUnmarshal)
	buf.WriteString("\treturn off, nil\n}\n\n")
	return true
}

type bodyKind int

const (
	bodySize bodyKind = iota
	bodyMarshal
	bodyUnmarshal
)

// emitBody 按条目发射一类方法体（size/marshal/unmarshal）。
func (g *gen) emitBody(buf *bytes.Buffer, name string, items []*item, kind bodyKind) {
	for _, it := range items {
		buf.WriteString("\t{\n")
		switch kind {
		case bodySize:
			g.emitSize(buf, name, it)
		case bodyMarshal:
			g.emitMarshal(buf, name, it)
		case bodyUnmarshal:
			g.emitUnmarshal(buf, name, it)
		}
		buf.WriteString("\t}\n")
	}
}

// emitStub 为布局非法的类型生成返回校验错误的 stub（Register 期 fail-fast）。
func (g *gen) emitStub(buf *bytes.Buffer, name, msg string, bad *fieldMeta) {
	field := ""
	if bad != nil {
		field = bad.name
	}
	err := fmt.Sprintf("%sLayoutErr(\"%%s\", %q, %q, %q)", g.q(""), name, field, msg)
	fmt.Fprintf(buf, "func (m *%s) CodecGenPtr() {}\n\n", name)
	fmt.Fprintf(buf, "func (m *%s) CodecSize() (int, error) {\n\treturn 0, %s\n}\n\n", name, sprintfErr(err, "marshal"))
	fmt.Fprintf(buf, "func (m *%s) CodecMarshalTo(dst []byte) int {\n\treturn 0\n}\n\n", name)
	fmt.Fprintf(buf, "func (m %s) CodecSizeVal() (int, error) {\n\treturn 0, %s\n}\n\n", name, sprintfErr(err, "marshal"))
	fmt.Fprintf(buf, "func (m %s) CodecEncodeInto(dst []byte) (int, error) {\n\treturn 0, %s\n}\n\n", name, sprintfErr(err, "marshal"))
	fmt.Fprintf(buf, "func (m *%s) CodecUnmarshalFrom(src []byte) (int, error) {\n\treturn 0, %s\n}\n\n", name, sprintfErr(err, "unmarshal"))
	fmt.Fprintf(buf, "func (m %s) CodecValidate() error {\n\treturn %s\n}\n\n", name, sprintfErr(err, "register"))
}

// sprintfErr 把 err 模板（第一个 %s 为 op）填上操作名。
func sprintfErr(tmpl, op string) string {
	return fmt.Sprintf(tmpl, op)
}

// --- 标量读写 ---

// readExpr 生成从 src[off:] 读 width 字节的表达式（转 named 类型）。
func (g *gen) readExpr(t types.Type, width int, signed bool) string {
	ref := g.typeRef(t)
	switch width {
	case 1:
		if signed {
			return fmt.Sprintf("%s(int8(src[off]))", ref)
		}
		return fmt.Sprintf("%s(src[off])", ref)
	case 2:
		if signed {
			return fmt.Sprintf("%s(int16(binary.LittleEndian.Uint16(src[off:])))", ref)
		}
		return fmt.Sprintf("%s(binary.LittleEndian.Uint16(src[off:]))", ref)
	case 4:
		if signed {
			return fmt.Sprintf("%s(int32(binary.LittleEndian.Uint32(src[off:])))", ref)
		}
		return fmt.Sprintf("%s(binary.LittleEndian.Uint32(src[off:]))", ref)
	default:
		if signed {
			return fmt.Sprintf("%s(int64(binary.LittleEndian.Uint64(src[off:])))", ref)
		}
		return fmt.Sprintf("%s(binary.LittleEndian.Uint64(src[off:]))", ref)
	}
}

// rawReadExpr 生成无类型转换的原始读（uint64/uint32/...）。
func rawReadExpr(width int) string {
	switch width {
	case 1:
		return "uint64(src[off])"
	case 2:
		return "uint64(binary.LittleEndian.Uint16(src[off:]))"
	case 4:
		return "uint64(binary.LittleEndian.Uint32(src[off:]))"
	default:
		return "binary.LittleEndian.Uint64(src[off:])"
	}
}

// writeStmt 生成把 val（Go 表达式）写入 dst[off:] 的语句。
func writeStmt(val string, width int, signed bool) string {
	switch width {
	case 1:
		return fmt.Sprintf("dst[off] = byte(%s)", val)
	case 2:
		if signed {
			return fmt.Sprintf("binary.LittleEndian.PutUint16(dst[off:], uint16(int16(%s)))", val)
		}
		return fmt.Sprintf("binary.LittleEndian.PutUint16(dst[off:], uint16(%s))", val)
	case 4:
		if signed {
			return fmt.Sprintf("binary.LittleEndian.PutUint32(dst[off:], uint32(int32(%s)))", val)
		}
		return fmt.Sprintf("binary.LittleEndian.PutUint32(dst[off:], uint32(%s))", val)
	default:
		if signed {
			return fmt.Sprintf("binary.LittleEndian.PutUint64(dst[off:], uint64(int64(%s)))", val)
		}
		return fmt.Sprintf("binary.LittleEndian.PutUint64(dst[off:], uint64(%s))", val)
	}
}

func (g *gen) truncGuard(buf *bytes.Buffer, name string, it *item, width int) {
	fmt.Fprintf(buf, "\tif off+%d > len(src) {\n\t\treturn 0, %sTruncErr(%q, %q, off, %d, len(src)-off)\n\t}\n",
		width, g.q(""), name, it.f.name, width)
}

// emitSize 发射 CodecSize 的条目语句。
func (g *gen) emitSize(buf *bytes.Buffer, name string, it *item) {
	switch it.kind {
	case kindScalar:
		fmt.Fprintf(buf, "\tn += %d\n", it.width)
	case kindArrayScalar, kindArrayStruct:
		if it.kind == kindArrayScalar {
			fmt.Fprintf(buf, "\tn += %d * %d\n", it.elemCount, it.elemWidth)
			return
		}
		fmt.Fprintf(buf, "\tfor i := range m.%s {\n", it.f.name)
		fmt.Fprintf(buf, "\t\tsz, err := m.%s[i].CodecSize()\n", it.f.name)
		fmt.Fprintf(buf, "\t\tif err != nil {\n\t\t\treturn 0, %sWrapErr(err, %q, %q, 0)\n\t\t}\n", g.q(""), name, it.f.name)
		buf.WriteString("\t\tn += sz\n\t}\n")
	case kindNested:
		fmt.Fprintf(buf, "\tsz, err := m.%s.CodecSize()\n", it.f.name)
		fmt.Fprintf(buf, "\tif err != nil {\n\t\treturn 0, %sWrapErr(err, %q, %q, 0)\n\t}\n", g.q(""), name, it.f.name)
		buf.WriteString("\tn += sz\n")
	case kindSkip:
		fmt.Fprintf(buf, "\tn += %d\n", it.skipN)
	case kindGreedy:
		fmt.Fprintf(buf, "\tn += len(m.%s)\n", it.f.name)
	case kindWireLenBlob, kindWireLenStruct, kindWireLenSlice:
		g.emitWireLenSize(buf, name, it)
	case kindCountMarker:
		fmt.Fprintf(buf, "\tn += %d\n", it.width)
	case kindCountList:
		g.emitCountListSize(buf, name, it)
	case kindPtrScalar, kindPtrStruct, kindPtrSlice, kindPtrArray:
		g.emitPtrSize(buf, name, it)
	}
}

func (g *gen) emitWireLenSize(buf *bytes.Buffer, name string, it *item) {
	wlType := g.typeRef(it.wl.typ)
	var wireLenExpr string
	switch it.kind {
	case kindWireLenBlob:
		wireLenExpr = fmt.Sprintf("len(m.%s)", it.target.name)
	case kindWireLenStruct:
		wireLenExpr = fmt.Sprintf("%d", it.targetFixed)
	case kindWireLenSlice:
		wireLenExpr = fmt.Sprintf("len(m.%s) * %d", it.target.name, it.elemWidth)
	}
	fmt.Fprintf(buf, "\twireLen := %s\n", wireLenExpr)
	if it.wlWidth < 4 {
		fmt.Fprintf(buf, "\tif wireLen > %d {\n\t\treturn 0, %sWireLenOverflowErr(%q, %q, wireLen, %d)\n\t}\n",
			1<<(8*it.wlWidth)-1, g.q(""), name, it.wl.name, it.wlWidth)
	}
	fmt.Fprintf(buf, "\tm.%s = %s(wireLen)\n", it.wl.name, wlType)
	fmt.Fprintf(buf, "\tn += %d + wireLen\n", it.wlWidth)
}

func (g *gen) emitCountListSize(buf *bytes.Buffer, name string, it *item) {
	ref := it.ref
	maxExpr := ""
	if ref.width < 8 {
		if ref.signed {
			maxExpr = fmt.Sprintf("%d", 1<<(8*ref.width-1)-1)
		} else {
			maxExpr = fmt.Sprintf("%d", 1<<(8*ref.width)-1)
		}
	}
	dst := "m." + ref.fieldName
	if ref.viaPtr {
		fmt.Fprintf(buf, "\tif m.%s == nil {\n\t\treturn 0, %sFieldErr(%q, %q, %q, 0, %q)\n\t}\n",
			ref.fieldName, g.q(""), "marshal", name, it.f.name, "count 目标字段为 nil 指针")
		dst = "*" + dst
	}
	cntType := g.countFieldTypeRef(it, ref)
	if maxExpr != "" {
		fmt.Fprintf(buf, "\tcnt := len(m.%s)\n", it.f.name)
		fmt.Fprintf(buf, "\tif cnt > %s {\n\t\treturn 0, %sCountOverflowErr(%q, %q, %q, cnt, %d, %v)\n\t}\n",
			maxExpr, g.q(""), name, it.f.name, ref.fieldName, ref.width, ref.signed)
	} else {
		fmt.Fprintf(buf, "\tcnt := len(m.%s)\n", it.f.name)
	}
	fmt.Fprintf(buf, "\t%s = %s(cnt)\n", dst, cntType)
	fmt.Fprintf(buf, "\tn += cnt * %d\n", it.elemWidth)
}

// countFieldTypeRef 计数字段的类型书写（经指针时取元素类型）。
func (g *gen) countFieldTypeRef(it *item, ref countRef) string {
	_ = it
	t := g.countFieldType(ref)
	return g.typeRef(t)
}

func (g *gen) countFieldType(ref countRef) types.Type {
	for _, f := range g.curMeta.fields {
		if f.name == ref.fieldName {
			ct := f.typ
			if ref.viaPtr {
				if p, ok := ct.Underlying().(*types.Pointer); ok {
					ct = p.Elem()
				}
			}
			return ct
		}
	}
	return nil
}

func (g *gen) emitPtrSize(buf *bytes.Buffer, name string, it *item) {
	f := it.f
	fmt.Fprintf(buf, "\tif m.%s == nil {\n", f.name)
	if it.cond == nil {
		fmt.Fprintf(buf, "\t\treturn 0, %sFieldErr(%q, %q, %q, 0, %q)\n", g.q(""), "marshal", name, f.name, "无条件指针字段为 nil")
	}
	buf.WriteString("\t} else {\n")
	switch it.kind {
	case kindPtrScalar:
		fmt.Fprintf(buf, "\t\tn += %d\n", it.width)
	case kindPtrStruct:
		fmt.Fprintf(buf, "\t\tsz, err := m.%s.CodecSize()\n", f.name)
		fmt.Fprintf(buf, "\t\tif err != nil {\n\t\t\treturn 0, %sWrapErr(err, %q, %q, 0)\n\t\t}\n", g.q(""), name, f.name)
		buf.WriteString("\t\tn += sz\n")
	case kindPtrArray:
		if it.elemIsStruct {
			fmt.Fprintf(buf, "\t\tfor i := range m.%s {\n", f.name)
			fmt.Fprintf(buf, "\t\t\tsz, err := m.%s[i].CodecSize()\n", f.name)
			fmt.Fprintf(buf, "\t\t\tif err != nil {\n\t\t\t\treturn 0, %sWrapErr(err, %q, %q, 0)\n\t\t\t}\n", g.q(""), name, f.name)
			buf.WriteString("\t\t\tn += sz\n\t\t}\n")
		} else {
			fmt.Fprintf(buf, "\t\tn += %d * %d\n", it.elemCount, it.elemWidth)
		}
	case kindPtrSlice:
		ref := it.ref
		dst := "m." + ref.fieldName
		if ref.viaPtr {
			fmt.Fprintf(buf, "\t\tif m.%s == nil {\n\t\t\treturn 0, %sFieldErr(%q, %q, %q, 0, %q)\n\t\t}\n",
				ref.fieldName, g.q(""), "unmarshal", name, f.name, "count 目标字段为 nil 指针")
			dst = "*" + dst
		}
		maxExpr := ""
		if ref.width < 8 {
			if ref.signed {
				maxExpr = fmt.Sprintf("%d", 1<<(8*ref.width-1)-1)
			} else {
				maxExpr = fmt.Sprintf("%d", 1<<(8*ref.width)-1)
			}
		}
		fmt.Fprintf(buf, "\t\tcnt := len(*m.%s)\n", f.name)
		if maxExpr != "" {
			fmt.Fprintf(buf, "\t\tif cnt > %s {\n\t\t\treturn 0, %sCountOverflowErr(%q, %q, %q, cnt, %d, %v)\n\t\t}\n",
				maxExpr, g.q(""), name, f.name, ref.fieldName, ref.width, ref.signed)
		}
		fmt.Fprintf(buf, "\t\t%s = %s(cnt)\n", dst, g.countFieldTypeRef(it, ref))
		fmt.Fprintf(buf, "\t\tn += cnt * %d\n", it.elemWidth)
	}
	buf.WriteString("\t}\n")
}

// emitMarshal 发射 CodecMarshalTo 的条目语句。
func (g *gen) emitMarshal(buf *bytes.Buffer, name string, it *item) {
	switch it.kind {
	case kindScalar:
		if it.isBool {
			fmt.Fprintf(buf, "\tif m.%s {\n\t\tdst[off] = 1\n\t} else {\n\t\tdst[off] = 0\n\t}\n", it.f.name)
		} else {
			buf.WriteString("\t" + writeStmt("m."+it.f.name, it.width, it.signed) + "\n")
		}
		fmt.Fprintf(buf, "\toff += %d\n", it.width)
	case kindArrayScalar:
		if it.elemIsByte {
			fmt.Fprintf(buf, "\tcopy(dst[off:], m.%s[:])\n", it.f.name)
			fmt.Fprintf(buf, "\toff += %d\n", it.elemCount*it.elemWidth)
		} else {
			fmt.Fprintf(buf, "\tfor i := range m.%s {\n", it.f.name)
			if it.elemIsBool {
				fmt.Fprintf(buf, "\t\tif m.%s[i] {\n\t\t\tdst[off] = 1\n\t\t} else {\n\t\t\tdst[off] = 0\n\t\t}\n", it.f.name)
			} else {
				buf.WriteString("\t\t" + writeStmt(fmt.Sprintf("m.%s[i]", it.f.name), it.elemWidth, it.elemSigned) + "\n")
			}
			fmt.Fprintf(buf, "\t\toff += %d\n", it.elemWidth)
			buf.WriteString("\t}\n")
		}
	case kindArrayStruct:
		fmt.Fprintf(buf, "\tfor i := range m.%s {\n", it.f.name)
		fmt.Fprintf(buf, "\t\toff += m.%s[i].CodecMarshalTo(dst[off:])\n", it.f.name)
		buf.WriteString("\t}\n")
	case kindNested:
		fmt.Fprintf(buf, "\toff += m.%s.CodecMarshalTo(dst[off:])\n", it.f.name)
	case kindSkip:
		fmt.Fprintf(buf, "\t// skip %d 字节（dst 已按预算清零）\n", it.skipN)
		fmt.Fprintf(buf, "\toff += %d\n", it.skipN)
	case kindGreedy:
		fmt.Fprintf(buf, "\tcopy(dst[off:], m.%s)\n", it.f.name)
		fmt.Fprintf(buf, "\toff += len(m.%s)\n", it.f.name)
	case kindWireLenBlob, kindWireLenStruct, kindWireLenSlice:
		g.emitWireLenMarshal(buf, name, it)
	case kindCountMarker:
		buf.WriteString("\t" + writeStmt("m."+it.f.name, it.width, false) + "\n")
		fmt.Fprintf(buf, "\toff += %d\n", it.width)
	case kindCountList:
		g.emitCountListMarshal(buf, name, it)
	case kindPtrScalar, kindPtrStruct, kindPtrSlice, kindPtrArray:
		g.emitPtrMarshal(buf, name, it)
	}
}

func (g *gen) emitWireLenMarshal(buf *bytes.Buffer, name string, it *item) {
	buf.WriteString("\t" + writeStmt("m."+it.wl.name, it.wlWidth, false) + "\n")
	fmt.Fprintf(buf, "\toff += %d\n", it.wlWidth)
	switch it.kind {
	case kindWireLenBlob:
		fmt.Fprintf(buf, "\tcopy(dst[off:], m.%s)\n", it.target.name)
		fmt.Fprintf(buf, "\toff += len(m.%s)\n", it.target.name)
	case kindWireLenStruct:
		fmt.Fprintf(buf, "\toff += m.%s.CodecMarshalTo(dst[off:])\n", it.target.name)
	case kindWireLenSlice:
		if it.elemIsStruct {
			fmt.Fprintf(buf, "\tfor i := range m.%s {\n", it.target.name)
			fmt.Fprintf(buf, "\t\toff += m.%s[i].CodecMarshalTo(dst[off:])\n", it.target.name)
			buf.WriteString("\t}\n")
		} else if it.elemIsByte {
			fmt.Fprintf(buf, "\tcopy(dst[off:], m.%s)\n", it.target.name)
			fmt.Fprintf(buf, "\toff += len(m.%s)\n", it.target.name)
		} else {
			fmt.Fprintf(buf, "\tfor i := range m.%s {\n", it.target.name)
			buf.WriteString("\t\t" + writeStmt(fmt.Sprintf("m.%s[i]", it.target.name), it.elemWidth, it.elemSigned) + "\n")
			buf.WriteString("\t\toff += " + fmt.Sprintf("%d", it.elemWidth) + "\n")
			buf.WriteString("\t}\n")
		}
	}
	_ = name
}

func (g *gen) emitCountListMarshal(buf *bytes.Buffer, name string, it *item) {
	if it.elemIsStruct {
		fmt.Fprintf(buf, "\tfor i := range m.%s {\n", it.f.name)
		fmt.Fprintf(buf, "\t\toff += m.%s[i].CodecMarshalTo(dst[off:])\n", it.f.name)
		buf.WriteString("\t}\n")
	} else if it.elemIsByte {
		fmt.Fprintf(buf, "\tcopy(dst[off:], m.%s)\n", it.f.name)
		fmt.Fprintf(buf, "\toff += len(m.%s)\n", it.f.name)
	} else {
		fmt.Fprintf(buf, "\tfor i := range m.%s {\n", it.f.name)
		if it.elemIsBool {
			fmt.Fprintf(buf, "\t\tif m.%s[i] {\n\t\t\tdst[off] = 1\n\t\t} else {\n\t\t\tdst[off] = 0\n\t\t}\n", it.f.name)
		} else {
			buf.WriteString("\t\t" + writeStmt(fmt.Sprintf("m.%s[i]", it.f.name), it.elemWidth, it.elemSigned) + "\n")
		}
		fmt.Fprintf(buf, "\t\toff += %d\n", it.elemWidth)
		buf.WriteString("\t}\n")
	}
	_ = name
}

func (g *gen) emitPtrMarshal(buf *bytes.Buffer, name string, it *item) {
	f := it.f
	fmt.Fprintf(buf, "\tif m.%s != nil {\n", f.name)
	switch it.kind {
	case kindPtrScalar:
		if it.isBool {
			fmt.Fprintf(buf, "\t\tif *m.%s {\n\t\t\tdst[off] = 1\n\t\t} else {\n\t\t\tdst[off] = 0\n\t\t}\n", f.name)
		} else {
			buf.WriteString("\t\t" + writeStmt("*m."+f.name, it.width, it.signed) + "\n")
		}
		fmt.Fprintf(buf, "\t\toff += %d\n", it.width)
	case kindPtrStruct:
		fmt.Fprintf(buf, "\t\toff += m.%s.CodecMarshalTo(dst[off:])\n", f.name)
	case kindPtrArray:
		if it.elemIsByte {
			fmt.Fprintf(buf, "\t\tcopy(dst[off:], m.%s[:])\n", f.name)
			fmt.Fprintf(buf, "\t\toff += %d\n", it.elemCount*it.elemWidth)
		} else {
			fmt.Fprintf(buf, "\t\tfor i := range m.%s {\n", f.name)
			buf.WriteString("\t\t\t" + writeStmt(fmt.Sprintf("m.%s[i]", f.name), it.elemWidth, it.elemSigned) + "\n")
			fmt.Fprintf(buf, "\t\t\toff += %d\n", it.elemWidth)
			buf.WriteString("\t\t}\n")
		}
	case kindPtrSlice:
		fmt.Fprintf(buf, "\t\tfor i := range *m.%s {\n", f.name)
		if it.elemIsBool {
			fmt.Fprintf(buf, "\t\t\tif (*m.%s)[i] {\n\t\t\t\tdst[off] = 1\n\t\t\t} else {\n\t\t\t\tdst[off] = 0\n\t\t\t}\n", f.name)
		} else {
			buf.WriteString("\t\t\t" + writeStmt(fmt.Sprintf("(*m.%s)[i]", f.name), it.elemWidth, it.elemSigned) + "\n")
		}
		fmt.Fprintf(buf, "\t\t\toff += %d\n", it.elemWidth)
		buf.WriteString("\t\t}\n")
	}
	buf.WriteString("\t}\n")
	_ = name
}

// emitUnmarshal 发射 CodecUnmarshalFrom 的条目语句。
func (g *gen) emitUnmarshal(buf *bytes.Buffer, name string, it *item) {
	switch it.kind {
	case kindScalar:
		g.truncGuard(buf, name, it, it.width)
		if it.isBool {
			fmt.Fprintf(buf, "\tm.%s = src[off] != 0\n", it.f.name)
		} else {
			fmt.Fprintf(buf, "\tm.%s = %s\n", it.f.name, g.readExpr(it.f.typ, it.width, it.signed))
		}
		fmt.Fprintf(buf, "\toff += %d\n", it.width)
	case kindArrayScalar:
		total := it.elemCount * it.elemWidth
		g.truncGuard(buf, name, it, total)
		if it.elemIsByte {
			fmt.Fprintf(buf, "\tcopy(m.%s[:], src[off:off+%d])\n", it.f.name, total)
		} else {
			fmt.Fprintf(buf, "\tfor i := range m.%s {\n", it.f.name)
			if it.elemIsBool {
				fmt.Fprintf(buf, "\t\tm.%s[i] = src[off+i] != 0\n", it.f.name)
			} else {
				fmt.Fprintf(buf, "\t\tm.%s[i] = %s\n", it.f.name, g.readExprAt(it.f.name, it.elemGoType, it.elemWidth, it.elemSigned, "off+i*"+fmt.Sprint(it.elemWidth)))
			}
			buf.WriteString("\t}\n")
		}
		fmt.Fprintf(buf, "\toff += %d\n", total)
	case kindArrayStruct:
		fmt.Fprintf(buf, "\tfor i := range m.%s {\n", it.f.name)
		fmt.Fprintf(buf, "\t\tn, err := m.%s[i].CodecUnmarshalFrom(src[off:])\n", it.f.name)
		fmt.Fprintf(buf, "\t\tif err != nil {\n\t\t\treturn 0, %sWrapErr(err, %q, %q, off)\n\t\t}\n", g.q(""), name, it.f.name)
		buf.WriteString("\t\toff += n\n\t}\n")
	case kindNested:
		fmt.Fprintf(buf, "\tn, err := m.%s.CodecUnmarshalFrom(src[off:])\n", it.f.name)
		fmt.Fprintf(buf, "\tif err != nil {\n\t\treturn 0, %sWrapErr(err, %q, %q, off)\n\t}\n", g.q(""), name, it.f.name)
		buf.WriteString("\toff += n\n")
	case kindSkip:
		g.truncGuard(buf, name, it, it.skipN)
		fmt.Fprintf(buf, "\toff += %d\n", it.skipN)
	case kindGreedy:
		buf.WriteString("\t{\n\t\tremain := len(src) - off\n")
		fmt.Fprintf(buf, "\tm.%s = make([]byte, remain)\n", it.f.name)
		fmt.Fprintf(buf, "\tcopy(m.%s, src[off:])\n", it.f.name)
		buf.WriteString("\toff = len(src)\n\t}\n")
	case kindWireLenBlob, kindWireLenStruct, kindWireLenSlice:
		g.emitWireLenUnmarshal(buf, name, it)
	case kindCountMarker:
		g.truncGuard(buf, name, it, it.width)
		fmt.Fprintf(buf, "\tm.%s = %s\n", it.f.name, g.readExpr(it.f.typ, it.width, false))
		fmt.Fprintf(buf, "\toff += %d\n", it.width)
	case kindCountList:
		g.emitCountListUnmarshal(buf, name, it)
	case kindPtrScalar, kindPtrStruct, kindPtrSlice, kindPtrArray:
		g.emitPtrUnmarshal(buf, name, it)
	}
}

// readExprAt 生成从 src[at] 读取元素的表达式。
func (g *gen) readExprAt(fieldName string, t types.Type, width int, signed bool, at string) string {
	ref := g.typeRef(t)
	switch width {
	case 1:
		if signed {
			return fmt.Sprintf("%s(int8(src[%s]))", ref, at)
		}
		return fmt.Sprintf("%s(src[%s])", ref, at)
	case 2:
		if signed {
			return fmt.Sprintf("%s(int16(binary.LittleEndian.Uint16(src[%s:])))", ref, at)
		}
		return fmt.Sprintf("%s(binary.LittleEndian.Uint16(src[%s:]))", ref, at)
	case 4:
		if signed {
			return fmt.Sprintf("%s(int32(binary.LittleEndian.Uint32(src[%s:])))", ref, at)
		}
		return fmt.Sprintf("%s(binary.LittleEndian.Uint32(src[%s:]))", ref, at)
	default:
		if signed {
			return fmt.Sprintf("%s(int64(binary.LittleEndian.Uint64(src[%s:])))", ref, at)
		}
		return fmt.Sprintf("%s(binary.LittleEndian.Uint64(src[%s:]))", ref, at)
	}
}

func (g *gen) emitWireLenUnmarshal(buf *bytes.Buffer, name string, it *item) {
	wlType := g.typeRef(it.wl.typ)
	g.truncGuardWL(buf, name, it)
	fmt.Fprintf(buf, "\twireLen := int(%s)\n", rawReadExpr(it.wlWidth))
	fmt.Fprintf(buf, "\tm.%s = %s(wireLen)\n", it.wl.name, wlType)
	fmt.Fprintf(buf, "\toff += %d\n", it.wlWidth)
	target := it.target.name
	switch it.kind {
	case kindWireLenBlob:
		fmt.Fprintf(buf, "\tif wireLen > len(src)-off {\n\t\treturn 0, %sTruncErr(%q, %q, off, wireLen, len(src)-off)\n\t}\n", g.q(""), name, target)
		fmt.Fprintf(buf, "\tm.%s = make([]byte, wireLen)\n", target)
		fmt.Fprintf(buf, "\tcopy(m.%s, src[off:off+wireLen])\n", target)
		buf.WriteString("\toff += wireLen\n")
	case kindWireLenStruct:
		fmt.Fprintf(buf, "\tif wireLen != %d {\n\t\treturn 0, %sWireLenMismatchErr(%q, %q, wireLen, %d)\n\t}\n",
			it.targetFixed, g.q(""), name, target, it.targetFixed)
		fmt.Fprintf(buf, "\tn, err := m.%s.CodecUnmarshalFrom(src[off:])\n", target)
		fmt.Fprintf(buf, "\tif err != nil {\n\t\treturn 0, %sWrapErr(err, %q, %q, off)\n\t}\n", g.q(""), name, target)
		buf.WriteString("\toff += n\n")
	case kindWireLenSlice:
		fmt.Fprintf(buf, "\tif wireLen > len(src)-off {\n\t\treturn 0, %sTruncErr(%q, %q, off, wireLen, len(src)-off)\n\t}\n", g.q(""), name, target)
		if it.elemWidth > 0 {
			fmt.Fprintf(buf, "\tif wireLen%%%d != 0 {\n\t\treturn 0, %sWireLenNotMultipleErr(%q, %q, wireLen, %d)\n\t}\n",
				it.elemWidth, g.q(""), name, target, it.elemWidth)
		}
		fmt.Fprintf(buf, "\tcnt := wireLen / %d\n", it.elemWidth)
		fmt.Fprintf(buf, "\tm.%s = make([]%s, cnt)\n", target, it.elemTypeName)
		if it.elemIsStruct {
			fmt.Fprintf(buf, "\tfor i := range m.%s {\n", target)
			fmt.Fprintf(buf, "\t\tn, err := m.%s[i].CodecUnmarshalFrom(src[off:])\n", target)
			fmt.Fprintf(buf, "\t\tif err != nil {\n\t\t\treturn 0, %sWrapErr(err, %q, %q, off)\n\t\t}\n", g.q(""), name, target)
			buf.WriteString("\t\toff += n\n\t}\n")
		} else if it.elemIsByte {
			fmt.Fprintf(buf, "\tcopy(m.%s, src[off:off+wireLen])\n", target)
			buf.WriteString("\toff += wireLen\n")
		} else {
			fmt.Fprintf(buf, "\tfor i := range m.%s {\n", target)
			if it.elemIsBool {
				fmt.Fprintf(buf, "\t\tm.%s[i] = src[off+i] != 0\n", target)
			} else {
				fmt.Fprintf(buf, "\t\tm.%s[i] = %s\n", target, g.readExprAt(target, it.elemGoType, it.elemWidth, it.elemSigned, fmt.Sprintf("off+i*%d", it.elemWidth)))
			}
			buf.WriteString("\t}\n")
			buf.WriteString("\toff += wireLen\n")
		}
	}
}

func (g *gen) truncGuardWL(buf *bytes.Buffer, name string, it *item) {
	fmt.Fprintf(buf, "\tif off+%d > len(src) {\n\t\treturn 0, %sTruncErr(%q, %q, off, %d, len(src)-off)\n\t}\n",
		it.wlWidth, g.q(""), name, it.wl.name, it.wlWidth)
}

func (g *gen) emitCountListUnmarshal(buf *bytes.Buffer, name string, it *item) {
	ref := it.ref
	cntExpr := "m." + ref.fieldName
	if ref.viaPtr {
		fmt.Fprintf(buf, "\tif m.%s == nil {\n\t\treturn 0, %sFieldErr(%q, %q, %q, 0, %q)\n\t}\n",
			ref.fieldName, g.q(""), "marshal", name, it.f.name, "count 目标字段为 nil 指针")
		cntExpr = "*m." + ref.fieldName
	}
	fmt.Fprintf(buf, "\tcnt := int64(%s)\n", cntExpr)
	fmt.Fprintf(buf, "\tif cnt < 0 || cnt > 1<<24 {\n\t\treturn 0, %sBadLenErr(%q, %q, off, cnt)\n\t}\n", g.q(""), name, it.f.name)
	fmt.Fprintf(buf, "\tneed := int(cnt) * %d\n", it.elemWidth)
	fmt.Fprintf(buf, "\tif need > len(src)-off {\n\t\treturn 0, %sTruncErr(%q, %q, off, need, len(src)-off)\n\t}\n", g.q(""), name, it.f.name)
	fmt.Fprintf(buf, "\tm.%s = make([]%s, int(cnt))\n", it.f.name, it.elemTypeName)
	if it.elemIsStruct {
		fmt.Fprintf(buf, "\tfor i := range m.%s {\n", it.f.name)
		fmt.Fprintf(buf, "\t\tn, err := m.%s[i].CodecUnmarshalFrom(src[off:])\n", it.f.name)
		fmt.Fprintf(buf, "\t\tif err != nil {\n\t\t\treturn 0, %sWrapErr(err, %q, %q, off)\n\t\t}\n", g.q(""), name, it.f.name)
		buf.WriteString("\t\toff += n\n\t}\n")
	} else if it.elemIsByte {
		fmt.Fprintf(buf, "\tcopy(m.%s, src[off:off+need])\n", it.f.name)
		buf.WriteString("\toff += need\n")
	} else {
		fmt.Fprintf(buf, "\tfor i := range m.%s {\n", it.f.name)
		if it.elemIsBool {
			fmt.Fprintf(buf, "\t\tm.%s[i] = src[off+i] != 0\n", it.f.name)
		} else {
			fmt.Fprintf(buf, "\t\tm.%s[i] = %s\n", it.f.name, g.readExprAt(it.f.name, it.elemGoType, it.elemWidth, it.elemSigned, fmt.Sprintf("off+i*%d", it.elemWidth)))
		}
		buf.WriteString("\t}\n")
		buf.WriteString("\toff += need\n")
	}
}

func (g *gen) emitPtrUnmarshal(buf *bytes.Buffer, name string, it *item) {
	f := it.f
	cond := "true"
	if it.cond != nil {
		cond = g.condExpr(it.cond)
	}
	fmt.Fprintf(buf, "\tif !(%s) {\n", cond)
	fmt.Fprintf(buf, "\t\tm.%s = nil\n", f.name)
	buf.WriteString("\t} else {\n")
	switch it.kind {
	case kindPtrScalar:
		g.truncGuardP(buf, name, f, it.width)
		fmt.Fprintf(buf, "\t\tv := %s\n", g.readExpr(elemOfPtr(f.typ), it.width, it.signed))
		fmt.Fprintf(buf, "\t\tm.%s = &v\n", f.name)
		fmt.Fprintf(buf, "\t\toff += %d\n", it.width)
	case kindPtrStruct:
		fmt.Fprintf(buf, "\t\tv := %s{}\n", it.targetTypeName)
		buf.WriteString("\t\tn, err := v.CodecUnmarshalFrom(src[off:])\n")
		fmt.Fprintf(buf, "\t\tif err != nil {\n\t\t\treturn 0, %sWrapErr(err, %q, %q, off)\n\t\t}\n", g.q(""), name, f.name)
		fmt.Fprintf(buf, "\t\tm.%s = &v\n", f.name)
		buf.WriteString("\t\toff += n\n")
	case kindPtrArray:
		total := it.elemCount * it.elemWidth
		fmt.Fprintf(buf, "\t\tif off+%d > len(src) {\n\t\t\treturn 0, %sTruncErr(%q, %q, off, %d, len(src)-off)\n\t\t}\n",
			total, g.q(""), name, f.name, total)
		fmt.Fprintf(buf, "\t\tv := %s{}\n", g.ptrArrayType(f))
		if it.elemIsByte {
			fmt.Fprintf(buf, "\t\tcopy(v[:], src[off:off+%d])\n", total)
		} else {
			fmt.Fprintf(buf, "\t\tfor i := range v {\n")
			fmt.Fprintf(buf, "\t\t\tv[i] = %s\n", g.readExprAt(f.name, it.elemGoType, it.elemWidth, it.elemSigned, fmt.Sprintf("off+i*%d", it.elemWidth)))
			buf.WriteString("\t\t}\n")
		}
		fmt.Fprintf(buf, "\t\tm.%s = &v\n", f.name)
		fmt.Fprintf(buf, "\t\toff += %d\n", total)
	case kindPtrSlice:
		ref := it.ref
		cntExpr := "m." + ref.fieldName
		if ref.viaPtr {
			fmt.Fprintf(buf, "\t\tif m.%s == nil {\n\t\t\treturn 0, %sFieldErr(%q, %q, %q, 0, %q)\n\t\t}\n",
				ref.fieldName, g.q(""), "unmarshal", name, f.name, "count 目标字段为 nil 指针")
			cntExpr = "*m." + ref.fieldName
		}
		fmt.Fprintf(buf, "\t\tcnt := int64(%s)\n", cntExpr)
		fmt.Fprintf(buf, "\t\tif cnt < 0 || cnt > 1<<24 {\n\t\t\treturn 0, %sBadLenErr(%q, %q, off, cnt)\n\t\t}\n", g.q(""), name, f.name)
		fmt.Fprintf(buf, "\t\tneed := int(cnt) * %d\n", it.elemWidth)
		fmt.Fprintf(buf, "\t\tif need > len(src)-off {\n\t\t\treturn 0, %sTruncErr(%q, %q, off, need, len(src)-off)\n\t\t}\n", g.q(""), name, f.name)
		fmt.Fprintf(buf, "\t\tv := make([]%s, int(cnt))\n", it.elemTypeName)
		if it.elemIsStruct {
			buf.WriteString("\t\tfor i := range v {\n")
			buf.WriteString("\t\t\tn, err := v[i].CodecUnmarshalFrom(src[off:])\n")
			fmt.Fprintf(buf, "\t\t\tif err != nil {\n\t\t\t\treturn 0, %sWrapErr(err, %q, %q, off)\n\t\t\t}\n", g.q(""), name, f.name)
			buf.WriteString("\t\t\toff += n\n\t\t}\n")
		} else if it.elemIsByte {
			buf.WriteString("\t\tcopy(v, src[off:off+need])\n")
		} else {
			buf.WriteString("\t\tfor i := range v {\n")
			if it.elemIsBool {
				buf.WriteString("\t\t\tv[i] = src[off+i] != 0\n")
			} else {
				fmt.Fprintf(buf, "\t\t\tv[i] = %s\n", g.readExprAt(f.name, it.elemGoType, it.elemWidth, it.elemSigned, fmt.Sprintf("off+i*%d", it.elemWidth)))
			}
			buf.WriteString("\t\t}\n")
		}
		fmt.Fprintf(buf, "\t\tm.%s = &v\n", f.name)
		buf.WriteString("\t\toff += need\n")
	}
	buf.WriteString("\t}\n")
}

func (g *gen) truncGuardP(buf *bytes.Buffer, name string, f *fieldMeta, width int) {
	fmt.Fprintf(buf, "\t\tif off+%d > len(src) {\n\t\t\treturn 0, %sTruncErr(%q, %q, off, %d, len(src)-off)\n\t\t}\n",
		width, g.q(""), name, f.name, width)
}

// condExpr 生成谓词表达式。
func (g *gen) condExpr(c *condRef) string {
	base := "m." + c.fieldName
	if c.viaPtr {
		base = "*(" + base + ")"
	}
	v := "uint64(" + base + ")"
	if c.signed {
		v = "uint64(int64(" + base + "))"
	}
	if c.maskAll {
		return v + " != 0"
	}
	return fmt.Sprintf("%s&0x%x != 0", v, c.mask)
}

// ptrArrayType 指针字段的数组类型书写（*elem 经 f.typ）。
func (g *gen) ptrArrayType(f *fieldMeta) string {
	return g.typeRef(f.typ)
}
