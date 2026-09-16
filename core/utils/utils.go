package utils

// NullTerminate 将任意类型的切片的最后一个元素设置为零值
// 这是对C++模板函数的Go语言实现：
// template<size_t N, typename T>
//
//	static void NullTerminate(T(&str)[N]) {
//	    str[N - 1] = 0;
//	}
func NullTerminate[T any](slice []T, zero T) {
	if len(slice) > 0 {
		slice[len(slice)-1] = zero
	}
}

// Wcscmp 泛型实现，支持多种字符类型
func Wcscmp[T ~uint16 | ~rune](s1, s2 []T) int {
	for i := 0; ; i++ {
		if i >= len(s1) || s1[i] == 0 {
			if i >= len(s2) || s2[i] == 0 {
				return 0
			}
			return -1
		}
		if i >= len(s2) || s2[i] == 0 {
			return 1
		}
		if s1[i] < s2[i] {
			return -1
		}
		if s1[i] > s2[i] {
			return 1
		}
	}
}
