//go:build !windows

package hw

func Detect() Info {
	return Recommend("", 0, 0, 0)
}
