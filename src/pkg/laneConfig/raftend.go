package laneConfig

type RaftEnds struct {
	Me        int       `yaml:"Me"`
	Endpoints []RaftEnd `yaml:"Endpoints"`
}

type RaftEnd struct {
	Addr string `yaml:"Addr"`
	Port string `yaml:"Port"`
}

func (c *RaftEnds) Default() {
	*c = DefaultRaftEnds()
}

func DefaultRaftEnds() RaftEnds {
	return RaftEnds{
		Me: 0,
		Endpoints: []RaftEnd{
			{
				Addr: "127.0.0.1",
				Port: ":32300",
			},
			{
				Addr: "127.0.0.1",
				Port: ":32301",
			},
			{
				Addr: "127.0.0.1",
				Port: ":32302",
			},
		},
	}
}
