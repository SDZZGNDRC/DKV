package laneConfig

type Kvserver struct {
	ServerAddr    string   `yaml:"ServerAddr"`
	ServerPort    string   `yaml:"ServerPort"`
	Rafts         RaftEnds `yaml:"Rafts"`
	DataBasePath  string   `yaml:"DataBasePath"`
	Maxraftstate  int      `yaml:"Maxraftstate"`
	AuthToken     string   `yaml:"AuthToken"`
	APIAuthTokens []string `yaml:"APIAuthTokens"`
	APIAddr       string   `yaml:"APIAddr"`
	APIPort       string   `yaml:"APIPort"`
}

func (c *Kvserver) Default() {
	*c = DefaultKVServer()
}

func DefaultKVServer() Kvserver {
	return Kvserver{
		ServerAddr:    "127.0.0.1",
		ServerPort:    ":51242",
		Rafts:         DefaultRaftEnds(),
		DataBasePath:  "data",
		Maxraftstate:  100000,
		AuthToken:     "123456789",
		APIAuthTokens: []string{"123456789"},
		APIAddr:       "0.0.0.0",
		APIPort:       ":5112",
	}
}
