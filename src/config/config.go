package config

import (
	"log"

	"github.com/SDZZGNDRC/DKV/src/raft"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type GlobalConfig struct {
	ServerAddr    string
	ServerPort    string
	Rafts         raft.RaftAddrs
	DataBasePath  string
	Maxraftstate  int
	AuthToken     string
	APIAuthTokens []string
	APIAddr       string
	APIPort       string
}

var Config *GlobalConfig

func init() {
	ConfigInstance := viper.New()

	pflag.Int("me", -1, "节点ID,用于覆盖配置文件中的Rafts.Me")
	pflag.String("serveraddr", "0.0.0.0", "Server Addr")
	pflag.String("serverport", ":5120", "Server Port")

	ConfigInstance.SetConfigName("config")
	ConfigInstance.SetConfigType("json")
	ConfigInstance.AddConfigPath(".")

	if err := ConfigInstance.ReadInConfig(); err != nil {
		log.Fatalf("无法读取配置文件: %v", err)
	}

	if meValue := pflag.Lookup("me"); meValue != nil && meValue.Changed {
		me, err := pflag.CommandLine.GetInt("me")
		if err == nil {
			ConfigInstance.Set("Rafts.Me", me)
		}
	}

	if serveraddrValue := pflag.Lookup("serveraddr"); serveraddrValue != nil && serveraddrValue.Changed {
		serveraddr, err := pflag.CommandLine.GetString("serveraddr")
		if err == nil {
			ConfigInstance.Set("ServerAddr", serveraddr)
		}
	}

	if serverportValue := pflag.Lookup("serverport"); serverportValue != nil && serverportValue.Changed {
		serverport, err := pflag.CommandLine.GetString("serverport")
		if err == nil {
			ConfigInstance.Set("ServerPort", serverport)
		}
	}

	Config = &GlobalConfig{}
	if err := ConfigInstance.Unmarshal(Config); err != nil {
		log.Fatalf("无法解析配置: %v", err)
	}
}
