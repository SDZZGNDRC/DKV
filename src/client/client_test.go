package client_test

import (
	"strings"
	"testing"

	"github.com/SDZZGNDRC/DKV/src/client"
)

// 定义测试用的服务器地址
var testServers = []string{
	"localhost:5110",
	"localhost:5111",
	"localhost:5112",
}

// 测试基础读写功能
func TestBasicPutGet(t *testing.T) {
	ck := client.MakeClerk(testServers, "123456789")

	t.Run("PutGet", func(t *testing.T) {
		ck.Put("name", "Alice")
		if val := ck.Get("name"); val != "Alice" {
			t.Errorf("期待='Alice', 实际='%s'", val)
		}
	})

	t.Run("Overwrite", func(t *testing.T) {
		ck.Put("name", "Bob")
		if val := ck.Get("name"); val != "Bob" {
			t.Errorf("期待='Bob', 实际='%s'", val)
		}
	})
}

// 测试追加功能
func TestAppendOperation(t *testing.T) {
	ck := client.MakeClerk(testServers, "123456789")

	ck.Put("msg", "Hello")
	ck.Append("msg", " World")

	if val := ck.Get("msg"); val != "Hello World" {
		t.Errorf("期待='Hello World', 实际='%s'", val)
	}
}

// 测试不存在的键
func TestNonExistentKey(t *testing.T) {
	ck := client.MakeClerk(testServers, "123456789")

	if val := ck.Get("nonexistent"); val != "" {
		t.Errorf("期待空值, 实际='%s'", val)
	}
}

// 测试无效token
func TestInvalidToken(t *testing.T) {
	ck := client.MakeClerk(testServers, "invalid_token")

	t.Run("PutWithInvalidToken", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("使用无效token应该导致错误")
			}
		}()
		ck.Put("test", "value")
	})

	t.Run("GetWithInvalidToken", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("使用无效token应该导致错误")
			}
		}()
		ck.Get("test")
	})
}

// 测试大数据操作
func TestLargeData(t *testing.T) {
	ck := client.MakeClerk(testServers, "123456789")

	// 生成1MB的数据
	largeValue := strings.Repeat("x", 1024*1024)

	t.Run("PutLargeData", func(t *testing.T) {
		ck.Put("large-key", largeValue)
		if val := ck.Get("large-key"); val != largeValue {
			t.Error("大数据读写不一致")
		}
	})
}

// 测试空值操作
func TestEmptyValues(t *testing.T) {
	ck := client.MakeClerk(testServers, "123456789")

	t.Run("PutEmptyValue", func(t *testing.T) {
		ck.Put("empty-key", "")
		if val := ck.Get("empty-key"); val != "" {
			t.Errorf("期待空值, 实际='%s'", val)
		}
	})

	t.Run("AppendToEmptyValue", func(t *testing.T) {
		ck.Put("append-empty", "")
		ck.Append("append-empty", "test")
		if val := ck.Get("append-empty"); val != "test" {
			t.Errorf("期待='test', 实际='%s'", val)
		}
	})
}
