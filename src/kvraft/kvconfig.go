package kvraft

import (
	"github.com/SDZZGNDRC/DKV/src/raft"
)

type Kvserver struct {
	Addr         string
	Port         string
	Rafts        raft.RaftAddrs
	DataBasePath string
	Maxraftstate int
}
