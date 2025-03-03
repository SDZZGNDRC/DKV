package kvraft

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/SDZZGNDRC/DKV/proto"
)

type KVClient struct {
	Valid bool
	Conn  pb.KvserverClient
	// Realconn *grpc.ClientConn
}

func NewKvClient(addr string) *KVClient {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		DPrintf("Dial failed ", err.Error())
		return nil
	}
	client := pb.NewKvserverClient(conn)

	ret := &KVClient{
		Valid: true,
		Conn:  client,
	}
	return ret
}
