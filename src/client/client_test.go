package client_test

import (
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
	ck := client.MakeClerk(testServers)

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
	ck := client.MakeClerk(testServers)

	ck.Put("msg", "Hello")
	ck.Append("msg", " World")

	if val := ck.Get("msg"); val != "Hello World" {
		t.Errorf("期待='Hello World', 实际='%s'", val)
	}
}

// 测试不存在的键
func TestNonExistentKey(t *testing.T) {
	ck := client.MakeClerk(testServers)

	if val := ck.Get("nonexistent"); val != "" {
		t.Errorf("期待空值, 实际='%s'", val)
	}
}

// 测试并发操作
// func TestConcurrentOperations(t *testing.T) {
// 	ck := client.MakeClerk(testServers)

// 	// 使用WaitGroup确保并发完成
// 	var wg sync.WaitGroup
// 	wg.Add(2)

// 	go func() {
// 		defer wg.Done()
// 		ck.Put("counter", "100")
// 	}()

// 	go func() {
// 		defer wg.Done()
// 		ck.Append("counter", "+50")
// 	}()

// 	wg.Wait()

// 	val := ck.Get("counter")
// 	if val != "100+50" {
// 		t.Errorf("期待='100+50', 实际='%s'", val)
// 	}
// }
