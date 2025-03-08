package kvraft

import (
	"bytes"
	"context"
	"encoding/gob"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SDZZGNDRC/DKV/src/raft"
	"github.com/SDZZGNDRC/DKV/src/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/SDZZGNDRC/DKV/proto"
)

const Debug = true

const (
	HandleOpTimeOut = time.Millisecond * 2000 // 超时为2s
)

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf("kv---"+format, a...)
	}
	return
}

func ServerLog(format string, a ...interface{}) {
	DPrintf("server "+format, a...)
}

type OType string

const (
	OPGet    OType = "Get"
	OPPut    OType = "Put"
	OPAppend OType = "Append"
)

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	OpType     OType
	Key        string
	Val        string
	Seq        uint64
	Identifier int64
}

type Result struct {
	LastSeq uint64
	Err     string
	Value   string
	ResTerm int
}

type KVServer struct {
	pb.UnimplementedKvserverServer

	mu         sync.Mutex
	me         int
	rf         *raft.Raft
	applyCh    chan raft.ApplyMsg
	dead       int32                // set by Kill()
	waiCh      map[int]*chan Result // 映射 startIndex->ch
	historyMap map[int64]*Result    // 映射 Identifier->*result

	maxraftstate int // snapshot if log grows this big
	maxMapLen    int
	db           map[string]string
	persister    *raft.Persister
	lastApplied  int // 日志中的最高索引

	grpc *grpc.Server
}

func (kv *KVServer) LogInfoReceive(opArgs *Op, logType int) {
	// logType:
	// 	0: 新的请求
	// 	1: 重复的请求
	// 	2: 旧的请求
	needPanic := false
	dateStr := ""
	if logType == 0 {
		dateStr = "新的"
	} else if logType == 1 {
		dateStr = "重复"
	} else {
		dateStr = "旧的"
		needPanic = true
	}
	switch opArgs.OpType {
	case OPGet:
		ServerLog("leader %v identifier %v Seq %v %sGet请求: Get(%v),\n", kv.me, opArgs.Identifier, opArgs.Seq, dateStr, opArgs.Key)
	case OPPut:
		ServerLog("leader %v identifier %v Seq %v %sPut请求: Put(%v,%v),\n", kv.me, opArgs.Identifier, opArgs.Seq, dateStr, opArgs.Key, opArgs.Val)
	case OPAppend:
		ServerLog("leader %v identifier %v Seq %v %sPut请求: Put(%v,%v),\n", kv.me, opArgs.Identifier, opArgs.Seq, dateStr, opArgs.Key, opArgs.Val)
	}

	if needPanic {
		panic("没有记录更早的请求的结果")
	}
}

func (kv *KVServer) LogInfoDBExecute(opArgs *Op, err string, res string) {
	switch opArgs.OpType {
	case OPGet:
		if err != "" {
			ServerLog("server %v DBExecute: identifier %v Seq %v DB执行Get请求: Get(%v), Err=%s\n", kv.me, opArgs.Identifier, opArgs.Seq, opArgs.Key, err)
		} else {
			ServerLog("server %v DBExecute: iidentifier %v Seq %v DB执行Get请求: Get(%v), res=%s\n", kv.me, opArgs.Identifier, opArgs.Seq, opArgs.Key, res)
		}
	case OPPut:
		if err != "" {
			ServerLog("server %v DBExecute: iidentifier %v Seq %v DB执行Put请求: Put(%v,%v), Err=%s\n", kv.me, opArgs.Identifier, opArgs.Seq, opArgs.Key, opArgs.Val, err)

		} else {
			ServerLog("server %v DBExecute: iidentifier %v Seq %v DB执行Put请求: Put(%v,%v), res=%s\n", kv.me, opArgs.Identifier, opArgs.Seq, opArgs.Key, opArgs.Val, res)
		}
	case OPAppend:
		if err != "" {
			ServerLog("server %v DBExecute: iidentifier %v Seq %v DB执行Append请求: Put(%v,%v), Err=%s\n", kv.me, opArgs.Identifier, opArgs.Seq, opArgs.Key, opArgs.Val, err)
		} else {
			ServerLog("server %v DBExecute: iidentifier %v Seq %v DB执行Append请求: Put(%v,%v), res=%s\n", kv.me, opArgs.Identifier, opArgs.Seq, opArgs.Key, opArgs.Val, res)
		}
	}
}

func (kv *KVServer) DBExecute(op *Op) (res Result) {
	// 调用该函数需要持有锁
	res.LastSeq = op.Seq
	switch op.OpType {
	case OPGet:
		val, exist := kv.db[op.Key]
		if exist {
			kv.LogInfoDBExecute(op, "", val)
			res.Value = val
			return
		} else {
			res.Err = ErrKeyNotExist
			res.Value = ""
			kv.LogInfoDBExecute(op, "", ErrKeyNotExist)
			return
		}
	case OPPut:
		kv.db[op.Key] = op.Val
		kv.LogInfoDBExecute(op, "", kv.db[op.Key])
		return
	case OPAppend:
		val, exist := kv.db[op.Key]
		if exist {
			kv.db[op.Key] = val + op.Val
			kv.LogInfoDBExecute(op, "", kv.db[op.Key])
			return
		} else {
			kv.db[op.Key] = op.Val
			kv.LogInfoDBExecute(op, "", kv.db[op.Key])
			return
		}
	}
	return
}

func OpToBytes(op Op) ([]byte, error) {
	var buf bytes.Buffer

	// 注册自定义类型（确保解码可用，若仅编码可省略）
	gob.Register(OType(""))

	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(op); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func BytesToOp(data []byte) (*Op, error) {
	buf := bytes.NewReader(data)
	gob.Register(OType("")) // 必须与编码端一致
	var op Op
	dec := gob.NewDecoder(buf)
	if err := dec.Decode(&op); err != nil { // 注意传递指针
		return nil, err
	}
	return &op, nil
}

func (kv *KVServer) HandleOp(opArgs *Op) (res Result) {
	// 先判断是否有历史记录
	kv.mu.Lock()
	if hisMap, exist := kv.historyMap[opArgs.Identifier]; exist && hisMap.LastSeq == opArgs.Seq {
		kv.mu.Unlock()
		ServerLog("leader %v HandleOp: identifier %v Seq %v 的请求: %s(%v, %v) 从历史记录返回\n", kv.me, opArgs.Identifier, opArgs.OpType, opArgs.Key, opArgs.Val)
		return *hisMap
	}
	kv.mu.Unlock()

	ServerLog("leader %v HandleOp: identifier %v Seq %v 的请求: %s(%v, %v) 准备调用Start\n", kv.me, opArgs.Identifier, opArgs.OpType, opArgs.Key, opArgs.Val)

	op_bytes, err := OpToBytes(*opArgs)
	if err != nil {
		panic(err)
	}
	startIndex, startTerm, isLeader := kv.rf.Start(op_bytes)
	if !isLeader {
		return Result{Err: ErrNotLeader, Value: ""}
	}

	kv.mu.Lock()

	// 直接覆盖之前记录的chan
	newCh := make(chan Result)
	kv.waiCh[startIndex] = &newCh
	ServerLog("leader %v HandleOp: identifier %v Seq %v 的请求: %s(%v, %v) 新建管道: %p\n", kv.me, opArgs.Identifier, opArgs.Seq, opArgs.OpType, opArgs.Key, opArgs.Val, &newCh)
	kv.mu.Unlock() // Start函数耗时较长, 先解锁

	defer func() {
		kv.mu.Lock()
		delete(kv.waiCh, startIndex)
		close(newCh)
		kv.mu.Unlock()
	}()

	// 等待消息到达或超时
	select {
	case <-time.After(HandleOpTimeOut):
		res.Err = ErrHandleOpTimeOut
		ServerLog("server %v identifier %v Seq %v: 超时", kv.me, opArgs.Identifier, opArgs.Seq)
		return
	case msg, success := <-newCh:
		if success && msg.ResTerm == startTerm {
			res = msg
			ServerLog("server %v HandleOp: identifier %v Seq %v: HandleOp 成功, %s(%v, %v), res=%v", kv.me, opArgs.Identifier, opArgs.Seq, opArgs.OpType, opArgs.Key, opArgs.Val, res.Value)
			return
		} else if !success {
			// 通道已经关闭, 有另一个协程收到了消息 或 通道被更新的RPC覆盖
			// TODO: 是否需要判断消息到达时自己已经不是leader了?
			ServerLog("server %v HandleOp: identifier %v Seq %v: 通道已经关闭, 有另一个协程收到了消息 或 更新的RPC覆盖, args.OpType=%v, args.Key=%+v", kv.me, opArgs.Identifier, opArgs.Seq, opArgs.OpType, opArgs.Key)
			res.Err = ErrChanClose
			return
		} else {
			// term与一开始不匹配, 说明这个Leader可能过期了
			ServerLog("server %v HandleOp: identifier %v Seq %v: term与一开始不匹配, 说明这个Leader可能过期了, res.ResTerm=%v, startTerm=%+v", kv.me, opArgs.Identifier, opArgs.Seq, res.ResTerm, startTerm)
			res.Err = ErrLeaderOutDated
			res.Value = ""
			return
		}
	}
}

func (kv *KVServer) Get(_ context.Context, args *pb.GetArgs) (reply *pb.GetReply, err error) {
	reply = &pb.GetReply{}
	opArgs := &Op{OpType: OPGet, Seq: args.Seq, Key: args.Key, Identifier: args.Identifier}

	res := kv.HandleOp(opArgs)
	reply.Err = res.Err
	reply.Value = res.Value

	return reply, nil
}

func (kv *KVServer) PutAppend(_ context.Context, args *pb.PutAppendArgs) (reply *pb.PutAppendReply, err error) {
	reply = &pb.PutAppendReply{}
	opArgs := &Op{Seq: args.Seq, Key: args.Key, Val: args.Value, Identifier: args.Identifier}
	if args.Op == "Put" {
		opArgs.OpType = OPPut
	} else {
		opArgs.OpType = OPAppend
	}

	res := kv.HandleOp(opArgs)
	reply.Err = res.Err

	return reply, nil
}

func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	kv.rf.Kill()
	// Your code here, if desired.
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

func (kv *KVServer) ApplyHandler() {
	for !kv.killed() {
		log := <-kv.applyCh
		if log.CommandValid {
			decodedOp, err := BytesToOp(log.Command)
			if err != nil {
				panic(err)
			}
			kv.mu.Lock()

			// 如果在follower一侧, 可能这个log包含在快照中, 直接跳过
			if log.CommandIndex <= kv.lastApplied {
				kv.mu.Unlock()
				continue
			}

			kv.lastApplied = log.CommandIndex

			// 需要判断这个log是否需要被再次应用
			var res Result

			needApply := false
			if hisMap, exist := kv.historyMap[decodedOp.Identifier]; exist {
				if hisMap.LastSeq == decodedOp.Seq {
					// 历史记录存在且Seq相同, 直接套用历史记录
					res = *hisMap
				} else if hisMap.LastSeq < decodedOp.Seq {
					// 否则新建
					needApply = true
				}
			} else {
				// 历史记录不存在
				needApply = true
			}

			if needApply {
				// 执行log
				res = kv.DBExecute(decodedOp)
				res.ResTerm = log.SnapshotTerm

				// 更新历史记录
				kv.historyMap[decodedOp.Identifier] = &res
			}

			// Leader还需要额外通知handler处理clerk回复
			ch, exist := kv.waiCh[log.CommandIndex]
			if exist {
				kv.mu.Unlock()
				// 发送消息
				func() {
					defer func() {
						if recover() != nil {
							// 如果这里有 panic，是因为通道关闭
							ServerLog("leader %v ApplyHandler: 发现 identifier %v Seq %v 的管道不存在, 应该是超时被关闭了", kv.me, decodedOp.Identifier, decodedOp.Seq)
						}
					}()
					res.ResTerm = log.SnapshotTerm

					*ch <- res
				}()
				kv.mu.Lock()
			}

			// 每收到一个log就检测是否需要生成快照
			if kv.maxraftstate != -1 && kv.persister.RaftStateSize() >= kv.maxraftstate/100*95 {
				// 当达到95%容量时需要生成快照
				snapShot := kv.GenSnapShot()
				kv.rf.Snapshot(log.CommandIndex, snapShot)
			}
			kv.mu.Unlock()
		} else if log.SnapshotValid {
			// 日志项是一个快照
			kv.mu.Lock()
			if log.SnapshotIndex >= kv.lastApplied {
				kv.LoadSnapShot(log.Snapshot)
				kv.lastApplied = log.SnapshotIndex
			}
			kv.mu.Unlock()
		}
	}
}

func (kv *KVServer) GenSnapShot() []byte {
	// 调用时必须持有锁mu
	w := new(bytes.Buffer)
	e := gob.NewEncoder(w)

	e.Encode(kv.db)
	e.Encode(kv.historyMap)

	serverState := w.Bytes()
	return serverState
}

func (kv *KVServer) LoadSnapShot(snapShot []byte) {
	// 调用时必须持有锁mu
	if len(snapShot) == 0 || snapShot == nil {
		ServerLog("server %v LoadSnapShot: 快照为空", kv.me)
		return
	}

	r := bytes.NewBuffer(snapShot)
	d := gob.NewDecoder(r)

	tmpDB := make(map[string]string)
	tmpHistoryMap := make(map[int64]*Result)
	if d.Decode(&tmpDB) != nil ||
		d.Decode(&tmpHistoryMap) != nil {
		ServerLog("server %v LoadSnapShot 加载快照失败\n", kv.me)
	} else {
		kv.db = tmpDB
		kv.historyMap = tmpHistoryMap
		ServerLog("server %v LoadSnapShot 加载快照成功\n", kv.me)
	}
}

func (kv *KVServer) GetSysStatus(reqChan chan struct{}, respChan chan *types.SysStatus) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	for range reqChan {
		respChan <- &types.SysStatus{
			Role:      kv.rf.GetRole(),
			ServerId:  kv.me,
			Term:      kv.rf.GetCurrentTerm(),
			VotedFor:  kv.rf.GetVotedFor(),
			Timestamp: time.Now().Unix(),
		}
	}
}

func validateToken(token string, authToken string) bool {
	// 这里可以添加更复杂的token验证逻辑
	return token == authToken
}

func StartKVServer(conf Kvserver, me int, persister *raft.Persister, maxraftstate int, apiChan *types.APIChans) *KVServer {
	gob.Register(Op{})

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(me, persister, kv.applyCh, conf.Rafts)
	kv.persister = persister

	// You may need initialization code here.
	kv.historyMap = make(map[int64]*Result)
	kv.db = make(map[string]string)
	kv.waiCh = make(map[int]*chan Result)

	// 先在启动时检查是否有快照
	kv.mu.Lock()
	kv.LoadSnapShot(persister.ReadSnapshot())
	kv.mu.Unlock()

	go kv.ApplyHandler()

	// server grpc
	lis, err := net.Listen("tcp", conf.ServerAddr+conf.ServerPort)
	if err != nil {
		DPrintf("error: etcd start faild", err)
	}

	// 创建一个带有Token认证的拦截器
	authInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "无法获取元数据")
		}

		// 从metadata中获取token
		token := md.Get("token")
		if len(token) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "缺少认证token")
		}

		// 验证token
		if !validateToken(token[0], conf.AuthToken) {
			return nil, status.Errorf(codes.Unauthenticated, "无效的token")
		}

		return handler(ctx, req)
	}

	// 使用Token认证拦截器创建gRPC服务器
	gServer := grpc.NewServer(
		grpc.UnaryInterceptor(authInterceptor),
	)

	pb.RegisterKvserverServer(gServer, kv)
	go func() {
		if err := gServer.Serve(lis); err != nil {
			DPrintf("failed to serve : ", err.Error())
		}
	}()

	DPrintf("etcd serivce is running on addr: %s", conf.ServerAddr+conf.ServerPort)
	kv.grpc = gServer

	// start api goroutine
	go kv.GetSysStatus(apiChan.GetSysStatusReqChan, apiChan.GetSysStatusRespChan)

	DPrintf("server [%d] restart", kv.me)

	return kv
}
