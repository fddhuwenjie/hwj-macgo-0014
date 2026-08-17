package recovery_test

import (
	"testing"
)

func FuzzRecoveryDecode(f *testing.F) {
	f.Add([]byte("dummy"))
	f.Fuzz(func(t *testing.T, data []byte) {
		// 调用恢复解析逻辑，确保不崩溃
	})
}
