package main

import (
	"log"

	"github.com/SDZZGNDRC/DKV/src/config"
	"github.com/SDZZGNDRC/DKV/src/controller/api"
	"github.com/SDZZGNDRC/DKV/src/kvraft"
	"github.com/SDZZGNDRC/DKV/src/raft"
	"github.com/SDZZGNDRC/DKV/src/types"
)

func main() {
	apiChans := types.APIChans{
		GetSysStatusReqChan:  make(chan struct{}),
		GetSysStatusRespChan: make(chan *types.SysStatus),

		OpGetReqChan:  make(chan *string),
		OpGetRespChan: make(chan *string),

		OpAppendReqChan:  make(chan *types.OpAppendReq),
		OpAppendRespChan: make(chan *types.OpAppendResp),

		OpPutReqChan:  make(chan *types.OpPutReq),
		OpPutRespChan: make(chan *types.OpPutResp),
	}

	log.Println("Starting API server on", config.Config.APIAddr+config.Config.APIPort)
	api.InitAPI(config.Config.APIAddr+config.Config.APIPort, apiChans)
	log.Println("Starting KV server on", config.Config.ServerAddr+config.Config.ServerPort)
	kvraft.StartKVServer(config.Config, config.Config.Rafts.Me, raft.MakePersister(), config.Config.Maxraftstate, &apiChans)
	select {}
}
