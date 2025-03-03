package main

import (
	"fmt"
	"log"

	"github.com/SDZZGNDRC/DKV/src/kvraft"
	"github.com/SDZZGNDRC/DKV/src/raft"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	ConfigPath string
)

func init() {
	// 定义命令行参数
	pflag.Int("me", -1, "节点ID,用于覆盖配置文件中的Rafts.Me")
	pflag.Parse()
}

// LoadConfig 从指定路径加载KV服务器配置
func LoadConfig(configPath string) (*kvraft.Kvserver, error) {
	// 初始化viper
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("json")
	v.AddConfigPath(configPath)

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("无法读取配置文件: %w", err)
	}

	// 检查命令行是否设置了me参数
	meValue := pflag.Lookup("me")
	if meValue != nil && meValue.Changed {
		me, err := pflag.CommandLine.GetInt("me")
		if err == nil {
			// 直接设置到viper中，覆盖配置文件中的值
			v.Set("Rafts.Me", me)
		}
	}

	// 将配置映射到结构体
	var config kvraft.Kvserver
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("无法解析配置: %w", err)
	}

	return &config, nil
}

func main() {
	conf, err := LoadConfig(".")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	_ = kvraft.StartKVServer(*conf, conf.Rafts.Me, raft.MakePersister(), conf.Maxraftstate)
	select {}
}
