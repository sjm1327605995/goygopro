package main

import "fmt"

// okResult 构造 {"success": true, ...键值对} 的 Wails 绑定返回。
// kv 必须为成对的 key(string)/value；奇数长度或非 string 键属于编程错误，直接 panic。
func okResult(kv ...any) map[string]interface{} {
	m := make(map[string]interface{}, 1+len(kv)/2)
	m["success"] = true
	for i := 0; i < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			panic("okResult: key must be a string")
		}
		m[key] = kv[i+1] // 奇数长度时此处越界 panic
	}
	return m
}

// failResult 构造 {"success": false, "error": msg}。
func failResult(err error) map[string]interface{} {
	return map[string]interface{}{"success": false, "error": err.Error()}
}

// failResultf 构造 {"success": false, "error": fmt.Sprintf(format, args...)}。
func failResultf(format string, args ...any) map[string]interface{} {
	return map[string]interface{}{"success": false, "error": fmt.Sprintf(format, args...)}
}
