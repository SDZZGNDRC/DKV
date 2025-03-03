package client

import (
	"context"
	"crypto/rand"
	"math/big"
	"time"

	pb "github.com/SDZZGNDRC/DKV/proto"
	"github.com/SDZZGNDRC/DKV/src/kvraft"
)

const (
	RpcRetryInterval = time.Millisecond * 50
)

type Clerk struct {
	servers    []*kvraft.KVClient
	seq        uint64
	identifier int64
	leaderId   int
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func (ck *Clerk) GetSeq() (SendSeq uint64) {
	SendSeq = ck.seq
	ck.seq += 1
	return
}

func MakeClerk(servers []string) *Clerk {
	ck := new(Clerk)
	ck.servers = make([]*kvraft.KVClient, len(servers))

	for i := range ck.servers {
		ck.servers[i] = kvraft.NewKvClient(servers[i])
	}

	ck.identifier = nrand()
	ck.seq = 0
	return ck
}

// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
func (ck *Clerk) Get(key string) string {
	args := &pb.GetArgs{Key: key, Seq: ck.GetSeq(), Identifier: ck.identifier}

	for {
		reply := &pb.GetReply{}
		reply, err := ck.servers[ck.leaderId].Conn.Get(context.Background(), args)
		if err != nil || reply.Err == kvraft.ErrNotLeader || reply.Err == kvraft.ErrLeaderOutDated {
			if err != nil {
				reply.Err = kvraft.ERRRPCFailed
			}
			if reply.Err != kvraft.ErrNotLeader {
				kvraft.DPrintf("clerk %v Seq %v 重试Get(%v), Err=%s", args.Identifier, args.Key, args.Key, reply.Err)
			}

			ck.leaderId += 1
			ck.leaderId %= len(ck.servers)
			time.Sleep(RpcRetryInterval)
			continue
		}

		switch reply.Err {
		case kvraft.ErrChanClose:
			kvraft.DPrintf("clerk %v Seq %v 重试Get(%v), Err=%s", args.Identifier, args.Key, args.Key, reply.Err)
			time.Sleep(time.Microsecond * 5)
			continue
		case kvraft.ErrHandleOpTimeOut:
			kvraft.DPrintf("clerk %v Seq %v 重试Get(%v), Err=%s", args.Identifier, args.Key, args.Key, reply.Err)
			time.Sleep(RpcRetryInterval)
			continue
		case kvraft.ErrKeyNotExist:
			kvraft.DPrintf("clerk %v Seq %v 成功: Get(%v)=%v, Err=%s", args.Identifier, args.Key, args.Key, reply.Value, reply.Err)
			return reply.Value
		}
		kvraft.DPrintf("clerk %v Seq %v 成功: Get(%v)=%v, Err=%s", args.Identifier, args.Key, args.Key, reply.Value, reply.Err)

		return reply.Value
	}
}

// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
func (ck *Clerk) PutAppend(key string, value string, op string) {
	// You will have to modify this function.
	args := &pb.PutAppendArgs{Key: key, Value: value, Op: op, Seq: ck.GetSeq(), Identifier: ck.identifier}

	for {
		reply := &pb.PutAppendReply{}
		reply, err := ck.servers[ck.leaderId].Conn.PutAppend(context.Background(), args)
		if err != nil || reply.Err == kvraft.ErrNotLeader || reply.Err == kvraft.ErrLeaderOutDated {
			if err != nil {
				reply.Err = kvraft.ERRRPCFailed
			}
			if reply.Err != kvraft.ErrNotLeader {
				kvraft.DPrintf("clerk %v Seq %v 重试%s(%v, %v), Err=%s", args.Identifier, args.Key, args.Op, args.Key, args.Value, reply.Err)
			}

			ck.leaderId += 1
			ck.leaderId %= len(ck.servers)
			time.Sleep(RpcRetryInterval)
			continue
		}

		switch reply.Err {
		case kvraft.ErrChanClose:
			kvraft.DPrintf("clerk %v Seq %v 重试%s(%v, %v), Err=%s", args.Identifier, args.Key, args.Op, args.Key, args.Value, reply.Err)
			time.Sleep(RpcRetryInterval)
			continue
		case kvraft.ErrHandleOpTimeOut:
			kvraft.DPrintf("clerk %v Seq %v 重试%s(%v, %v), Err=%s", args.Identifier, args.Key, args.Op, args.Key, args.Value, reply.Err)
			time.Sleep(RpcRetryInterval)
			continue
		}
		kvraft.DPrintf("clerk %v Seq %v 成功: %s(%v, %v), Err=%s", args.Identifier, args.Key, args.Op, args.Key, args.Value, reply.Err)

		return
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
