package raft

import (
	pb "github.com/SDZZGNDRC/DKV/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type RaftAddrs struct {
	Me        int
	Endpoints []RaftAddr
}

type RaftAddr struct {
	Addr string
	Port string
}

type RaftEnd struct {
	conf RaftAddr
	conn pb.RaftClient
}

func NewRaftClient(conf RaftAddr) *RaftEnd {
	conn, err := grpc.NewClient(conf.Addr+conf.Port, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		DPrintf("Dail failed ", err.Error())
		return nil
	}
	DPrintf("Dial success %s", conf.Addr+conf.Port)
	client := pb.NewRaftClient(conn)
	ret := &RaftEnd{
		conn: client,
		conf: conf,
	}
	return ret
}
