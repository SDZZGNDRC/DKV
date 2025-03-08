package main

import (
	"flag"
	"runtime"

	"github.com/SDZZGNDRC/DKV/src/kvraft"
	"github.com/SDZZGNDRC/DKV/src/pkg/laneConfig"
	"github.com/SDZZGNDRC/DKV/src/pkg/laneLog"
	"github.com/SDZZGNDRC/DKV/src/raft"
)

var (
	ConfigPath = flag.String("c", "config.yml", "path fo config.yml folder")
)

func main() {
	runtime.GOMAXPROCS(1)
	flag.Parse()
	conf := laneConfig.Kvserver{}
	laneConfig.Init(*ConfigPath, &conf)
	if len(conf.Rafts.Endpoints)%2 == 0 {
		laneLog.Logger.Fatalln("the number of nodes", len(conf.Rafts.Endpoints), "is not odd")
	}
	if len(conf.Rafts.Endpoints) < 3 {
		laneLog.Logger.Fatalln("the number of nodes", len(conf.Rafts.Endpoints), "is less than 3")
	}
	// conf.Endpoints[conf.Me].Addr+conf.Endpoints[conf.Me].Addr

	laneLog.InitLogger("kvserver", true, false, false)
	_ = kvraft.StartKVServer(conf, conf.Rafts.Me, raft.MakePersister("/raftstate.dat", "/snapshot.dat", conf.DataBasePath), conf.Maxraftstate)
	select {}
}
