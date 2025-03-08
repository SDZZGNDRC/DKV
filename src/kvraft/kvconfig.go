package kvraft

import (
	"github.com/SDZZGNDRC/DKV/src/raft"
)

type Kvserver struct {
	ServerAddr   string
	ServerPort   string
	Rafts        raft.RaftAddrs
	DataBasePath string
	Maxraftstate int
	AuthToken    string
	APIAddr      string
	APIPort      string
}
