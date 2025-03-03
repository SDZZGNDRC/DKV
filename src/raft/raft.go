package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	//	"bytes"

	"bytes"
	"context"
	"encoding/gob"
	"log"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"

	pb "github.com/SDZZGNDRC/DKV/proto"
)

// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 2D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
type ApplyMsg struct {
	CommandValid bool
	Command      []byte
	CommandIndex int

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

const (
	Follower = iota
	Candidate
	Leader
)

const (
	HeartBeatTimeOut = 101
	ElectTimeOutBase = 450
)

// A Go object implementing a single Raft peer.
type Raft struct {
	pb.UnimplementedRaftServer

	mu        sync.Mutex // Lock to protect shared access to this peer's state
	peers     []*RaftEnd // RPC end points of all peers
	persister *Persister // Object to hold this peer's persisted state
	me        int        // this peer's index into peers[]
	dead      int32      // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	currentTerm int
	votedFor    int
	log         []pb.Entry

	nextIndex  []int
	matchIndex []int

	// 以下不是Figure 2中的field
	voteTimer  *time.Timer
	heartTimer *time.Timer
	rd         *rand.Rand
	role       int

	commitIndex int
	lastApplied int
	applyCh     chan ApplyMsg

	condApply *sync.Cond

	// 2D
	snapShot          []byte // 快照
	lastIncludedIndex int    // 日志中的最高索引
	lastIncludedTerm  int    // 日志中的最高Term
}

func (rf *Raft) Print() {
	DPrintf("raft%v:{currentTerm=%v, role=%v, votedFor=%v}\n", rf.me, rf.currentTerm, rf.role, rf.votedFor)
}

func (rf *Raft) ResetVoteTimer() {
	rdTimeOut := GetRandomElectTimeOut(rf.rd)
	rf.voteTimer.Reset(time.Duration(rdTimeOut) * time.Millisecond)
}

func (rf *Raft) ResetHeartTimer(timeStamp int) {
	rf.heartTimer.Reset(time.Duration(timeStamp) * time.Millisecond)
}

func (rf *Raft) RealLogIdx(vIdx int) int {
	// 调用该函数需要是加锁的状态
	return vIdx - rf.lastIncludedIndex
}

func (rf *Raft) VirtualLogIdx(rIdx int) int {
	// 调用该函数需要是加锁的状态
	return rIdx + rf.lastIncludedIndex
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	// Your code here (2A).
	rf.mu.Lock()
	// DPrintf("server %v GetState 获取锁mu", rf.me)
	defer func() {
		rf.mu.Unlock()
		// DPrintf("server %v GetState 释放锁mu", rf.me)
	}()
	return rf.currentTerm, rf.role == Leader
}

// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
// before you've implemented snapshots, you should pass nil as the
// second argument to persister.Save().
// after you've implemented snapshots, pass the current snapshot
// (or nil if there's not yet a snapshot).
func (rf *Raft) persist() {
	// TODO: 持久化lastIncludedIndex和lastIncludedTerm时, 是否需要加锁?
	// DPrintf("server %v 开始持久化, 最后一个持久化的log为: %v:%v", rf.me, len(rf.log)-1, rf.log[len(rf.log)-1].Cmd)

	w := new(bytes.Buffer)
	e := gob.NewEncoder(w)
	// 2C
	e.Encode(rf.votedFor)
	e.Encode(rf.currentTerm)
	e.Encode(rf.log)
	// 2D
	e.Encode(rf.lastIncludedIndex)
	e.Encode(rf.lastIncludedTerm)
	raftstate := w.Bytes()

	rf.persister.Save(raftstate, rf.snapShot)
}

// restore previously persisted state.
func (rf *Raft) readPersist(data []byte) {
	// 目前只在Make中调用, 因此不需要锁
	if len(data) == 0 {
		return
	}
	r := bytes.NewBuffer(data)
	d := gob.NewDecoder(r)

	var votedFor int
	var currentTerm int
	var log []pb.Entry
	var lastIncludedIndex int
	var lastIncludedTerm int
	if d.Decode(&votedFor) != nil ||
		d.Decode(&currentTerm) != nil ||
		d.Decode(&log) != nil ||
		d.Decode(&lastIncludedIndex) != nil ||
		d.Decode(&lastIncludedTerm) != nil {
		DPrintf("server %v readPersist failed\n", rf.me)
	} else {
		// 2C
		rf.votedFor = votedFor
		rf.currentTerm = currentTerm
		rf.log = log
		// 2D
		rf.lastIncludedIndex = lastIncludedIndex
		rf.lastIncludedTerm = lastIncludedTerm

		rf.commitIndex = lastIncludedIndex
		rf.lastApplied = lastIncludedIndex
		DPrintf("server %v  readPersist 成功\n", rf.me)
	}
}

func (rf *Raft) readSnapshot(data []byte) {
	// 目前只在Make中调用, 因此不需要锁
	if len(data) == 0 {
		DPrintf("server %v 读取快照失败: 无快照\n", rf.me)
		return
	}
	rf.snapShot = data
	DPrintf("server %v 读取快照c成功\n", rf.me)
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.commitIndex < index || index <= rf.lastIncludedIndex {
		DPrintf("server %v 拒绝了 Snapshot 请求, 其index=%v, 自身commitIndex=%v, lastIncludedIndex=%v\n", rf.me, index, rf.commitIndex, rf.lastIncludedIndex)
		return
	}

	DPrintf("server %v 同意了 Snapshot 请求, 其index=%v, 自身commitIndex=%v, 原来的lastIncludedIndex=%v, 快照后的lastIncludedIndex=%v\n", rf.me, index, rf.commitIndex, rf.lastIncludedIndex, index)

	// 保存snapshot
	rf.snapShot = snapshot

	rf.lastIncludedTerm = int(rf.log[rf.RealLogIdx(index)].Term)
	// 截断log
	rf.log = rf.log[rf.RealLogIdx(index):] // index位置的log被存在0索引处
	rf.lastIncludedIndex = index
	if rf.lastApplied < index {
		rf.lastApplied = index
	}

	rf.persist()
}

// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
func (rf *Raft) Start(command []byte) (int, int, bool) {
	// Your code here (2B).
	// 如果不是leader返回false
	rf.mu.Lock()
	// DPrintf("server %v Start 获取锁mu", rf.me)
	defer func() {
		// DPrintf("server %v Start 释放锁mu", rf.me)
		rf.ResetHeartTimer(15)
		rf.mu.Unlock()
	}()
	if rf.role != Leader {
		return -1, -1, false
	}

	rf.log = append(rf.log, pb.Entry{Term: int64(rf.currentTerm), Cmd: command})
	DPrintf("leader %v 准备持久化", rf.me)
	rf.persist()

	return rf.VirtualLogIdx(len(rf.log) - 1), rf.currentTerm, true
}

func (rf *Raft) CommitChecker() {
	// 检查是否有新的commit
	DPrintf("server %v 的 CommitChecker 开始运行", rf.me)
	for !rf.killed() {
		rf.mu.Lock()
		// DPrintf("server %v CommitChecker 获取锁mu", rf.me)
		for rf.commitIndex <= rf.lastApplied {
			rf.condApply.Wait()
		}
		msgBuf := make([]*ApplyMsg, 0, rf.commitIndex-rf.lastApplied)
		tmpApplied := rf.lastApplied
		for rf.commitIndex > tmpApplied {
			tmpApplied += 1
			if tmpApplied <= rf.lastIncludedIndex {
				// tmpApplied可能是snapShot中已经被截断的日志项, 这些日志项就不需要再发送了
				continue
			}
			if rf.RealLogIdx(tmpApplied) >= len(rf.log) {
				DPrintf("server %v CommitChecker数组越界: tmpApplied=%v,  rf.RealLogIdx(tmpApplied)=%v>=len(rf.log)=%v, lastIncludedIndex=%v", rf.me, tmpApplied, rf.RealLogIdx(tmpApplied), len(rf.log), rf.lastIncludedIndex)
			}
			msg := &ApplyMsg{
				CommandValid: true,
				Command:      rf.log[rf.RealLogIdx(tmpApplied)].Cmd,
				CommandIndex: tmpApplied,
				SnapshotTerm: int(rf.log[rf.RealLogIdx(tmpApplied)].Term),
			}

			msgBuf = append(msgBuf, msg)
		}
		rf.mu.Unlock()
		// DPrintf("server %v CommitChecker 释放锁mu", rf.me)

		// 注意, 在解锁后可能又出现了SnapShot进而修改了rf.lastApplied
		for _, msg := range msgBuf {
			rf.mu.Lock()
			if msg.CommandIndex != rf.lastApplied+1 {
				rf.mu.Unlock()
				continue
			}
			DPrintf("server %v 准备commit, log = %v:%v, lastIncludedIndex=%v", rf.me, msg.CommandIndex, msg.SnapshotTerm, rf.lastIncludedIndex)

			rf.mu.Unlock()
			// 注意, 在解锁后可能又出现了SnapShot进而修改了rf.lastApplied

			rf.applyCh <- *msg

			rf.mu.Lock()
			if msg.CommandIndex != rf.lastApplied+1 {
				rf.mu.Unlock()
				continue
			}
			rf.lastApplied = msg.CommandIndex
			rf.mu.Unlock()
		}
	}
}

func (rf *Raft) sendInstallSnapshot(serverTo int, args *pb.InstallSnapshotArgs) (reply *pb.InstallSnapshotReply, ok bool) {
	reply, err := rf.peers[serverTo].conn.InstallSnapshot(context.Background(), args)
	return reply, err == nil
}

// InstallSnapshot handler
func (rf *Raft) InstallSnapshot(_ context.Context, args *pb.InstallSnapshotArgs) (reply *pb.InstallSnapshotReply, err error) {
	reply = &pb.InstallSnapshotReply{}
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// DPrintf("server %v InstallSnapshot 获取锁mu", rf.me)

	// 1. Reply immediately if term < currentTerm
	if args.Term < int64(rf.currentTerm) {
		reply.Term = int64(rf.currentTerm)
		DPrintf("server %v 拒绝来自 %v 的 InstallSnapshot, 更小的Term\n", rf.me, args.LeaderId)

		return reply, nil
	}

	// 不需要实现分块的RPC

	if int(args.Term) > rf.currentTerm {
		rf.currentTerm = int(args.Term)
		rf.votedFor = -1
		DPrintf("server %v 接受来自 %v 的 InstallSnapshot, 且发现了更大的Term\n", rf.me, args.LeaderId)
	}

	rf.role = Follower
	rf.ResetVoteTimer()
	DPrintf("server %v 接收到 leader %v 的InstallSnapshot, 重设定时器", rf.me, args.LeaderId)

	if args.LastIncludedIndex < int64(rf.lastIncludedIndex) || args.LastIncludedIndex < int64(rf.commitIndex) {
		// 1. 快照反而比当前的 lastIncludedIndex 更旧, 不需要快照
		// 2. 快照比当前的 commitIndex 更旧, 不能安装快照
		reply.Term = int64(rf.currentTerm)
		return reply, nil
	}

	// 6. If existing log entry has same index and term as snapshot’s last included entry, retain log entries following it and reply
	hasEntry := false
	rIdx := 0
	for ; rIdx < len(rf.log); rIdx++ {
		if rf.VirtualLogIdx(rIdx) == int(args.LastIncludedIndex) && rf.log[rIdx].Term == args.LastIncludedTerm {
			hasEntry = true
			break
		}
	}

	msg := &ApplyMsg{
		SnapshotValid: true,
		Snapshot:      args.Data,
		SnapshotTerm:  int(args.LastIncludedTerm),
		SnapshotIndex: int(args.LastIncludedIndex),
	}

	if hasEntry {
		DPrintf("server %v InstallSnapshot: args.LastIncludedIndex= %v 位置存在, 保留后面的log\n", rf.me, args.LastIncludedIndex)

		rf.log = rf.log[rIdx:]
	} else {
		DPrintf("server %v InstallSnapshot: 清空log\n", rf.me)
		rf.log = make([]pb.Entry, 0)
		rf.log = append(rf.log, pb.Entry{Term: int64(rf.lastIncludedTerm), Cmd: args.LastIncludedCmd}) // 索引为0处占位
	}

	// 7. Discard the entire log
	// 8. Reset state machine using snapshot contents (and load snapshot’s cluster configuration)

	rf.snapShot = args.Data
	rf.lastIncludedIndex = int(args.LastIncludedIndex)
	rf.lastIncludedTerm = int(args.LastIncludedTerm)

	if rf.commitIndex < int(args.LastIncludedIndex) {
		rf.commitIndex = int(args.LastIncludedIndex)
	}

	if rf.lastApplied < int(args.LastIncludedIndex) {
		rf.lastApplied = int(args.LastIncludedIndex)
	}

	reply.Term = int64(rf.currentTerm)
	rf.applyCh <- *msg
	rf.persist()
	return reply, nil
}

func (rf *Raft) handleInstallSnapshot(serverTo int) {
	reply := &pb.InstallSnapshotReply{}

	rf.mu.Lock()
	// DPrintf("server %v handleInstallSnapshot 获取锁mu", rf.me)

	if rf.role != Leader {
		// 自己已经不是Lader了, 返回
		rf.mu.Unlock()
		return
	}

	args := &pb.InstallSnapshotArgs{
		Term:              int64(rf.currentTerm),
		LeaderId:          int64(rf.me),
		LastIncludedIndex: int64(rf.lastIncludedIndex),
		LastIncludedTerm:  int64(rf.lastIncludedTerm),
		Data:              rf.snapShot,
		LastIncludedCmd:   rf.log[0].Cmd,
	}

	rf.mu.Unlock()
	// DPrintf("server %v handleInstallSnapshot 释放锁mu", rf.me)

	// 发送RPC时不要持有锁
	reply, ok := rf.sendInstallSnapshot(serverTo, args)
	if !ok {
		// RPC发送失败, 下次再触发即可
		return
	}

	rf.mu.Lock()
	// DPrintf("server %v handleInstallSnapshot 获取锁mu", rf.me)
	defer func() {
		// DPrintf("server %v handleInstallSnapshot 释放锁mu", rf.me)
		rf.mu.Unlock()
	}()

	if rf.role != Leader || int64(rf.currentTerm) != args.Term {
		// 已经不是Leader或者是过期的Leader
		return
	}

	if reply.Term > int64(rf.currentTerm) {
		// 自己是旧Leader
		rf.currentTerm = int(reply.Term)
		rf.role = Follower
		rf.votedFor = -1
		rf.ResetVoteTimer()
		rf.persist()
		return
	}

	// LastIncludedIndex可能包括了还没有复制的日志项, 这些日志项可以不用复制了
	if rf.matchIndex[serverTo] < int(args.LastIncludedIndex) {
		rf.matchIndex[serverTo] = int(args.LastIncludedIndex)
	}
	rf.nextIndex[serverTo] = rf.matchIndex[serverTo] + 1
}

// example RequestVote RPC reply structure.
// field names must start with capital letters!

func (rf *Raft) sendAppendEntries(serverTo int, args *pb.AppendEntriesArgs) (reply *pb.AppendEntriesReply, ok bool) {
	reply, err := rf.peers[serverTo].conn.AppendEntries(context.Background(), args)
	return reply, err == nil
}

// AppendEntries handler
func (rf *Raft) AppendEntries(_ context.Context, args *pb.AppendEntriesArgs) (reply *pb.AppendEntriesReply, err error) {
	reply = &pb.AppendEntriesReply{}
	// Your code here (2A, 2B).
	// 新leader发送的第一个消息

	rf.mu.Lock()
	// DPrintf("server %v AppendEntries 获取锁mu", rf.me)
	defer func() {
		// DPrintf("server %v AppendEntries 释放锁mu", rf.me)
		rf.mu.Unlock()
	}()

	if args.Term < int64(rf.currentTerm) {
		// 1. Reply false if term < currentTerm (§5.1)
		// 有2种情况:
		// - 这是真正的来自旧的leader的消息
		// - 当前节点是一个孤立节点, 因为持续增加 currentTerm 进行选举, 因此真正的leader返回了更旧的term
		reply.Term = int64(rf.currentTerm)
		reply.Success = false
		DPrintf("server %v 收到了旧的leader% v 的心跳函数, args=%+v, 更新的term: %v\n", rf.me, args.LeaderId, args, reply.Term)
		return reply, nil
	}

	// 代码执行到这里就是 args.Term >= rf.currentTerm 的情况

	// 不是旧 leader的话需要记录访问时间
	rf.ResetVoteTimer()

	if int(args.Term) > rf.currentTerm {
		// 新leader的第一个消息
		rf.currentTerm = int(args.Term) // 更新iterm
		rf.votedFor = -1                // 易错点: 更新投票记录为未投票
		rf.role = Follower
		rf.persist()
	}

	if len(args.Entries) == 0 {
		// 心跳函数
		DPrintf("server %v 接收到 leader %v 的心跳, 自身lastIncludedIndex=%v, PrevLogIndex=%v, len(Entries) = %v\n", rf.me, args.LeaderId, rf.lastIncludedIndex, args.PrevLogIndex, len(args.Entries))
	} else {
		DPrintf("server %v 收到 leader %v 的AppendEntries, 自身lastIncludedIndex=%v, PrevLogIndex=%v, len(Entries)= %+v \n", rf.me, args.LeaderId, rf.lastIncludedIndex, args.PrevLogIndex, len(args.Entries))
	}

	isConflict := false

	// 校验PrevLogIndex和PrevLogTerm不合法
	// 2. Reply false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm (§5.3)
	if args.PrevLogIndex < int64(rf.lastIncludedIndex) {
		// 过时的RPC, 其 PrevLogIndex 甚至在lastIncludedIndex之前
		reply.Success = true
		reply.Term = int64(rf.currentTerm)
		return reply, nil
	} else if int(args.PrevLogIndex) >= rf.VirtualLogIdx(len(rf.log)) {
		// PrevLogIndex位置不存在日志项
		reply.XTerm = -1
		reply.XLen = int64(rf.VirtualLogIdx(len(rf.log))) // Log长度, 包括了已经snapShot的部分
		isConflict = true
		DPrintf("server %v 的log在PrevLogIndex: %v 位置不存在日志项, Log长度为%v\n", rf.me, args.PrevLogIndex, reply.XLen)
	} else if rf.log[rf.RealLogIdx(int(args.PrevLogIndex))].Term != args.PrevLogTerm {
		// PrevLogIndex位置的日志项存在, 但term不匹配
		reply.XTerm = rf.log[rf.RealLogIdx(int(args.PrevLogIndex))].Term
		i := args.PrevLogIndex
		for i > int64(rf.commitIndex) && rf.log[rf.RealLogIdx(int(i))].Term == reply.XTerm {
			i -= 1
		}
		reply.XIndex = i + 1
		reply.XLen = int64(rf.VirtualLogIdx(len(rf.log))) // Log长度, 包括了已经snapShot的部分
		isConflict = true
		DPrintf("server %v 的log在PrevLogIndex: %v 位置Term不匹配, args.Term=%v, 实际的term=%v\n", rf.me, args.PrevLogIndex, args.PrevLogTerm, reply.XTerm)
	}

	if isConflict {
		reply.Term = int64(rf.currentTerm)
		reply.Success = false
		return reply, nil
	}
	// 3. If an existing entry conflicts with a new one (same index
	// but different terms), delete the existing entry and all that
	// follow it (§5.3)
	// if len(args.Entries) != 0 && len(rf.log) > args.PrevLogIndex+1 && rf.log[args.PrevLogIndex+1].Term != args.Entries[0].Term {
	// 	// 发生了冲突, 移除冲突位置开始后面所有的内容
	// 	DPrintf("server %v 的log与args发生冲突, 进行移除\n", rf.me)
	// 	rf.log = rf.log[:args.PrevLogIndex+1]
	// }
	for idx, log := range args.Entries {
		ridx := rf.RealLogIdx(int(args.PrevLogIndex)) + 1 + idx
		if ridx < len(rf.log) && rf.log[ridx].Term != log.Term {
			// 某位置发生了冲突, 覆盖这个位置开始的所有内容
			rf.log = rf.log[:ridx]
			for _, entry := range args.Entries[idx:] {
				rf.log = append(rf.log, pb.Entry{Term: entry.Term, Cmd: entry.Cmd})
			}

			break
		} else if ridx == len(rf.log) {
			// 没有发生冲突但长度更长了, 直接拼接
			for _, entry := range args.Entries[idx:] {
				rf.log = append(rf.log, pb.Entry{Term: entry.Term, Cmd: entry.Cmd})
			}
			break
		}
	}
	if len(args.Entries) != 0 {
		DPrintf("server %v 成功进行apeend, lastApplied=%v, len(log)=%v\n", rf.me, rf.lastApplied, len(rf.log))
	}

	// 4. Append any new entries not already in the log
	// 补充apeend的业务
	rf.persist()

	reply.Success = true
	reply.Term = int64(rf.currentTerm)

	if args.LeaderCommit > int64(rf.commitIndex) {
		// 5.If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
		if int(args.LeaderCommit) > rf.VirtualLogIdx(len(rf.log)-1) {
			rf.commitIndex = rf.VirtualLogIdx(len(rf.log) - 1)
		} else {
			rf.commitIndex = int(args.LeaderCommit)
		}
		DPrintf("server %v 唤醒检查commit的协程, commitIndex=%v, len(log)=%v\n", rf.me, rf.commitIndex, len(rf.log))
		rf.condApply.Signal() // 唤醒检查commit的协程
	}
	return reply, nil
}

func (rf *Raft) handleAppendEntries(serverTo int, args *pb.AppendEntriesArgs) {
	// 目前的设计, 重试自动发生在下一次心跳函数, 所以这里不需要死循环

	reply, ok := rf.sendAppendEntries(serverTo, args)
	if !ok {
		return
	}

	rf.mu.Lock()
	// DPrintf("server %v handleAppendEntries 获取锁mu", rf.me)
	defer func() {
		// DPrintf("server %v handleAppendEntries 释放锁mu", rf.me)
		rf.mu.Unlock()
	}()

	if rf.role != Leader || args.Term != int64(rf.currentTerm) {
		// 函数调用间隙值变了, 已经不是发起这个调用时的term了
		// 要先判断term是否改变, 否则后续的更改matchIndex等是不安全的
		return
	}

	if reply.Success {
		// server回复成功
		newMatchIdx := int(args.PrevLogIndex) + len(args.Entries)
		if newMatchIdx > rf.matchIndex[serverTo] {
			// 有可能在此期间让follower安装了快照, 导致 rf.matchIndex[serverTo] 本来就更大
			rf.matchIndex[serverTo] = newMatchIdx
		}

		rf.nextIndex[serverTo] = rf.matchIndex[serverTo] + 1

		// 需要判断是否可以commit
		N := rf.VirtualLogIdx(len(rf.log) - 1)

		DPrintf("leader %v 确定N以决定新的commitIndex, lastIncludedIndex=%v, commitIndex=%v", rf.me, rf.lastIncludedIndex, rf.commitIndex)

		for N > rf.commitIndex {
			count := 1 // 1表示包括了leader自己
			for i := 0; i < len(rf.peers); i++ {
				if i == rf.me {
					continue
				}
				if rf.matchIndex[i] >= N && rf.log[rf.RealLogIdx(N)].Term == int64(rf.currentTerm) {
					// TODO: N有没有可能自减到snapShot之前的索引导致log出现负数索引越界?
					// 解答: 需要确保调用SnapShot时检查索引是否超过commitIndex
					count += 1
				}
			}
			if count > len(rf.peers)/2 {
				// +1 表示包括自身
				// 如果至少一半的follower回复了成功, 更新commitIndex
				break
			}
			N -= 1
		}

		rf.commitIndex = N
		rf.condApply.Signal() // 唤醒检查commit的协程

		return
	}

	if reply.Term > int64(rf.currentTerm) {
		// 回复了更新的term, 表示自己已经不是leader了
		DPrintf("server %v 旧的leader收到了来自 server % v 的心跳函数中更新的term: %v, 转化为Follower\n", rf.me, serverTo, reply.Term)

		rf.currentTerm = int(reply.Term)
		rf.role = Follower
		rf.votedFor = -1
		rf.ResetVoteTimer()
		rf.persist()
		return
	}

	if reply.Term == int64(rf.currentTerm) && rf.role == Leader {
		// term仍然相同, 且自己还是leader, 表名对应的follower在prevLogIndex位置没有与prevLogTerm匹配的项
		// 快速回退的处理
		if reply.XTerm == -1 {
			// PrevLogIndex这个位置在Follower中不存在
			DPrintf("leader %v 收到 server %v 的回退请求, 原因是log过短, 回退前的nextIndex[%v]=%v, 回退后的nextIndex[%v]=%v\n", rf.me, serverTo, serverTo, rf.nextIndex[serverTo], serverTo, reply.XLen)
			if rf.lastIncludedIndex >= int(reply.XLen) {
				// 由于snapshot被截断
				// 下一次心跳添加InstallSnapshot的处理
				rf.nextIndex[serverTo] = rf.lastIncludedIndex
			} else {
				rf.nextIndex[serverTo] = int(reply.XLen)
			}
			return
		}

		// 防止数组越界
		// if rf.nextIndex[serverTo] < 1 || rf.nextIndex[serverTo] >= len(rf.log) {
		// 	rf.nextIndex[serverTo] = 1
		// }
		i := rf.nextIndex[serverTo] - 1
		if i < rf.lastIncludedIndex {
			i = rf.lastIncludedIndex
		}
		for i > rf.lastIncludedIndex && rf.log[rf.RealLogIdx(i)].Term > reply.XTerm {
			i -= 1
		}

		if i == rf.lastIncludedIndex && rf.log[rf.RealLogIdx(i)].Term > reply.XTerm {
			// 要找的位置已经由于snapshot被截断
			// 下一次心跳添加InstallSnapshot的处理
			rf.nextIndex[serverTo] = rf.lastIncludedIndex
		} else if rf.log[rf.RealLogIdx(i)].Term == reply.XTerm {
			// 之前PrevLogIndex发生冲突位置时, Follower的Term自己也有

			DPrintf("leader %v 收到 server %v 的回退请求, 冲突位置的Term为%v, server的这个Term从索引%v开始, 而leader对应的最后一个XTerm索引为%v, 回退前的nextIndex[%v]=%v, 回退后的nextIndex[%v]=%v\n", rf.me, serverTo, reply.XTerm, reply.XIndex, i, serverTo, rf.nextIndex[serverTo], serverTo, i+1)
			rf.nextIndex[serverTo] = i + 1 // i + 1是确保没有被截断的
		} else {
			// 之前PrevLogIndex发生冲突位置时, Follower的Term自己没有
			DPrintf("leader %v 收到 server %v 的回退请求, 冲突位置的Term为%v, server的这个Term从索引%v开始, 而leader对应的XTerm不存在, 回退前的nextIndex[%v]=%v, 回退后的nextIndex[%v]=%v\n", rf.me, serverTo, reply.XTerm, reply.XIndex, serverTo, rf.nextIndex[serverTo], serverTo, reply.XIndex)
			if int(reply.XIndex) <= rf.lastIncludedIndex {
				// XIndex位置也被截断了
				// 添加InstallSnapshot的处理
				rf.nextIndex[serverTo] = rf.lastIncludedIndex
			} else {
				rf.nextIndex[serverTo] = int(reply.XIndex)
			}
		}
		return
	}
}

func (rf *Raft) SendHeartBeats() {
	// 2B相对2A的变化, 真实的AppendEntries也通过心跳发送
	DPrintf("leader %v 开始发送心跳\n", rf.me)

	for !rf.killed() {
		<-rf.heartTimer.C
		rf.mu.Lock()
		// DPrintf("server %v SendHeartBeats 获取锁mu", rf.me)
		// if the server is dead or is not the leader, just return
		if rf.role != Leader {
			rf.mu.Unlock()
			// DPrintf("server %v SendHeartBeats 释放锁mu", rf.me)
			// 不是leader则终止心跳的发送
			return
		}

		for i := 0; i < len(rf.peers); i++ {
			if i == rf.me {
				continue
			}
			args := &pb.AppendEntriesArgs{
				Term:         int64(rf.currentTerm),
				LeaderId:     int64(rf.me),
				PrevLogIndex: int64(rf.nextIndex[i] - 1),
				LeaderCommit: int64(rf.commitIndex),
			}

			sendInstallSnapshot := false

			if args.PrevLogIndex < int64(rf.lastIncludedIndex) {
				// 表示Follower有落后的部分且被截断, 改为发送同步心跳
				DPrintf("leader %v 取消向 server %v 广播新的心跳, 改为发送sendInstallSnapshot, lastIncludedIndex=%v, nextIndex[%v]=%v, args = %+v \n", rf.me, i, rf.lastIncludedIndex, i, rf.nextIndex[i], args)
				sendInstallSnapshot = true
			} else if rf.VirtualLogIdx(len(rf.log)-1) > int(args.PrevLogIndex) {
				// 如果有新的log需要发送, 则就是一个真正的AppendEntries而不是心跳

				startIdx := rf.RealLogIdx(int(args.PrevLogIndex) + 1)
				entries := make([]*pb.Entry, 0)
				for i := startIdx; i < len(rf.log); i++ {
					entries = append(entries, &rf.log[i])
				}
				args.Entries = entries
				DPrintf("leader %v 开始向 server %v 广播新的AppendEntries, lastIncludedIndex=%v, nextIndex[%v]=%v, PrevLogIndex=%v, len(Entries) = %v\n", rf.me, i, rf.lastIncludedIndex, i, rf.nextIndex[i], args.PrevLogIndex, len(args.Entries))
			} else {
				// 如果没有新的log发送, 就发送一个长度为0的切片, 表示心跳
				DPrintf("leader %v 开始向 server %v 广播新的心跳, lastIncludedIndex=%v, nextIndex[%v]=%v, PrevLogIndex=%v, len(Entries) = %v \n", rf.me, i, rf.lastIncludedIndex, i, rf.nextIndex[i], args.PrevLogIndex, len(args.Entries))
				args.Entries = make([]*pb.Entry, 0)
			}

			if sendInstallSnapshot {
				go rf.handleInstallSnapshot(i)
			} else {
				args.PrevLogTerm = rf.log[rf.RealLogIdx(int(args.PrevLogIndex))].Term
				go rf.handleAppendEntries(i, args)
			}
		}

		rf.mu.Unlock()
		// DPrintf("server %v SendHeartBeats 释放锁mu", rf.me)
		rf.ResetHeartTimer(HeartBeatTimeOut)
	}
}

// code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
func (rf *Raft) sendRequestVote(server int, args *pb.RequestVoteArgs) (reply *pb.RequestVoteReply, ok bool) {
	reply, err := rf.peers[server].conn.RequestVote(context.Background(), args)
	return reply, err == nil
}

// RequestVote RPC handler.
func (rf *Raft) RequestVote(_ context.Context, args *pb.RequestVoteArgs) (reply *pb.RequestVoteReply, err error) {
	// Your code here (2A, 2B).
	reply = &pb.RequestVoteReply{}

	rf.mu.Lock()
	// DPrintf("server %v RequestVote 获取锁mu", rf.me)
	defer func() {
		// DPrintf("server %v RequestVote 释放锁mu", rf.me)
		rf.mu.Unlock()
	}()

	if args.Term < int64(rf.currentTerm) {
		// 旧的term
		// 1. Reply false if term < currentTerm (§5.1)
		reply.Term = int64(rf.currentTerm)
		reply.VoteGranted = false
		DPrintf("server %v 拒绝向 server %v 投票: 旧的term: %v, args = %+v\n", rf.me, args.CandidateId, args.Term, args)
		return reply, nil
	}

	// 代码到这里时, args.Term >= rf.currentTerm

	if args.Term > int64(rf.currentTerm) {
		// 已经是新一轮的term, 之前的投票记录作废
		rf.currentTerm = int(args.Term) // 更新到更新的term
		rf.votedFor = -1
		rf.role = Follower
		rf.persist()
	}

	// at least as up-to-date as receiver’s log, grant vote (§5.2, §5.4)
	if rf.votedFor == -1 || int64(rf.votedFor) == args.CandidateId {
		// 首先确保是没投过票的
		if args.LastLogTerm > rf.log[len(rf.log)-1].Term ||
			(args.LastLogTerm == rf.log[len(rf.log)-1].Term && args.LastLogIndex >= int64(rf.VirtualLogIdx(len(rf.log)-1))) {
			// 2. If votedFor is null or candidateId, and candidate’s log is least as up-to-date as receiver’s log, grant vote (§5.2, §5.4)
			rf.currentTerm = int(args.Term)
			reply.Term = int64(rf.currentTerm)
			rf.votedFor = int(args.CandidateId)
			rf.role = Follower
			rf.ResetVoteTimer()
			rf.persist()

			reply.VoteGranted = true
			DPrintf("server %v 同意向 server %v 投票, args = %+v, len(rf.log)=%v\n", rf.me, args.CandidateId, args, len(rf.log))
			return
		} else {
			if args.LastLogTerm < rf.log[len(rf.log)-1].Term {
				DPrintf("server %v 拒绝向 server %v 投票: 更旧的LastLogTerm, args = %+v\n", rf.me, args.CandidateId, args)
			} else {
				DPrintf("server %v 拒绝向 server %v 投票: 更短的Log, args = %+v\n", rf.me, args.CandidateId, args)
			}
		}
	} else {
		DPrintf("server %v 拒绝向 server %v投票: 已投票, args = %+v\n", rf.me, args.CandidateId, args)
	}

	reply.Term = int64(rf.currentTerm)
	reply.VoteGranted = false
	return reply, nil
}

func (rf *Raft) GetVoteAnswer(server int, args *pb.RequestVoteArgs) bool {
	// sendArgs := *args
	// reply := pb.RequestVoteReply{}
	reply, ok := rf.sendRequestVote(server, args)
	if !ok {
		return false
	}

	rf.mu.Lock()
	// DPrintf("server %v GetVoteAnswer 获取锁mu", rf.me)
	defer func() {
		// DPrintf("server %v GetVoteAnswer 释放锁mu", rf.me)
		rf.mu.Unlock()
	}()

	if rf.role != Candidate || args.Term != int64(rf.currentTerm) {
		// 易错点: 函数调用的间隙被修改了
		return false
	}

	if reply.Term > int64(rf.currentTerm) {
		// 已经是过时的term了
		rf.currentTerm = int(reply.Term)
		rf.votedFor = -1
		rf.role = Follower
		rf.persist()
	}
	return reply.VoteGranted
}

func (rf *Raft) collectVote(serverTo int, args *pb.RequestVoteArgs, muVote *sync.Mutex, voteCount *int) {
	voteAnswer := rf.GetVoteAnswer(serverTo, args)
	if !voteAnswer {
		return
	}
	muVote.Lock()
	if *voteCount > len(rf.peers)/2 {
		muVote.Unlock()
		return
	}

	*voteCount += 1
	if *voteCount > len(rf.peers)/2 {
		rf.mu.Lock()
		// DPrintf("server %v collectVote 获取锁mu", rf.me)
		if rf.role != Candidate || rf.currentTerm != int(args.Term) {
			// 有另外一个投票的协程收到了更新的term而更改了自身状态为Follower
			// 或者自己的term已经过期了, 也就是被新一轮的选举追上了
			rf.mu.Unlock()
			// DPrintf("server %v 释放锁mu", rf.me)

			muVote.Unlock()
			return
		}
		DPrintf("server %v 成为了新的 leader", rf.me)
		rf.role = Leader
		// 需要重新初始化nextIndex和matchIndex
		for i := 0; i < len(rf.nextIndex); i++ {
			rf.nextIndex[i] = rf.VirtualLogIdx(len(rf.log))
			rf.matchIndex[i] = rf.lastIncludedIndex // 由于matchIndex初始化为lastIncludedIndex, 因此在崩溃恢复后, 大概率触发InstallSnapshot RPC
		}
		rf.mu.Unlock()
		// DPrintf("server %v collectVote 释放锁mu", rf.me)

		go rf.SendHeartBeats()
	}

	muVote.Unlock()
}

func (rf *Raft) Elect() {
	// 特别注意, 要先对muVote加锁, 再对mu加锁, 这是为了统一获取锁的顺序以避免死锁

	rf.mu.Lock()
	// DPrintf("server %v Elect 获取锁mu", rf.me)
	defer func() {
		// DPrintf("server %v Elect 释放锁mu", rf.me)
		rf.mu.Unlock()
	}()

	rf.currentTerm += 1 // 自增term
	rf.role = Candidate // 成为候选人
	rf.votedFor = rf.me // 给自己投票
	rf.persist()

	voteCount := 1 // 自己有一票
	var muVote sync.Mutex

	DPrintf("server %v 开始发起新一轮投票, 新一轮的term为: %v", rf.me, rf.currentTerm)

	args := &pb.RequestVoteArgs{
		Term:         int64(rf.currentTerm),
		CandidateId:  int64(rf.me),
		LastLogIndex: int64(rf.VirtualLogIdx(len(rf.log) - 1)),
		LastLogTerm:  rf.log[len(rf.log)-1].Term,
	}

	for i := 0; i < len(rf.peers); i++ {
		if i == rf.me {
			continue
		}
		go rf.collectVote(i, args, &muVote, &voteCount)
	}
}

func (rf *Raft) ticker() {
	for !rf.killed() {
		// Your code here (2A)
		// Check if a leader election should be started.

		// pause for a random amount of time between 50 and 350
		// milliseconds.
		<-rf.voteTimer.C
		rf.mu.Lock()
		// DPrintf("server %v ticker 获取锁mu", rf.me)
		if rf.role != Leader {
			// 超时
			go rf.Elect()
		}
		rf.ResetVoteTimer()
		rf.mu.Unlock()
		// DPrintf("server %v ticker 释放锁mu", rf.me)
	}
}

// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
func Make(me int,
	persister *Persister, applyCh chan ApplyMsg, raftAddrs RaftAddrs) *Raft {
	DPrintf("server %v 调用Make启动", me)

	rf := &Raft{}
	rf.persister = persister
	rf.me = me

	rf.log = make([]pb.Entry, 0)
	rf.log = append(rf.log, pb.Entry{Term: 0})

	rf.nextIndex = make([]int, len(raftAddrs.Endpoints))
	rf.matchIndex = make([]int, len(raftAddrs.Endpoints))
	// rf.timeStamp = time.Now()
	rf.role = Follower
	rf.applyCh = applyCh
	rf.condApply = sync.NewCond(&rf.mu)
	rf.rd = rand.New(rand.NewSource(int64(rf.me)))
	rf.voteTimer = time.NewTimer(0)
	rf.heartTimer = time.NewTimer(0)
	rf.ResetVoteTimer()

	// initialize from state persisted before a crash
	// 如果读取成功, 将覆盖log, votedFor和currentTerm
	rf.readSnapshot(persister.ReadSnapshot())
	rf.readPersist(persister.ReadRaftState())

	for i := 0; i < len(rf.nextIndex); i++ {
		rf.nextIndex[i] = rf.VirtualLogIdx(len(rf.log)) // raft中的index是从1开始的
	}

	// start ticker goroutine to start elections
	go rf.ticker()
	go rf.CommitChecker()

	rf.StartRaft(raftAddrs.Endpoints[me])
	servers := WaitConnect(raftAddrs)
	rf.peers = servers

	return rf
}

func (rt *Raft) StartRaft(conf RaftAddr) {
	// server grpc
	lis, err := net.Listen("tcp", conf.Addr+conf.Port)
	if err != nil {
		log.Fatalln("error: etcd start failed", err)
	}
	gServer := grpc.NewServer()
	pb.RegisterRaftServer(gServer, rt)
	go func() {
		if err := gServer.Serve(lis); err != nil {
			log.Fatalln("failed to serve : ", err.Error())
		}
	}()
}

func WaitConnect(conf RaftAddrs) []*RaftEnd {
	DPrintf("start waiting...")
	var wait sync.WaitGroup
	servers := make([]*RaftEnd, len(conf.Endpoints))
	wait.Add(len(servers) - 1)
	for i := range conf.Endpoints {
		if i == conf.Me {
			continue
		}

		go func(other int, conf RaftAddr) {
			defer wait.Done()
			for {
				r := NewRaftClient(conf)
				if r != nil {
					servers[other] = r
					break
				}
				time.Sleep(time.Millisecond * 500)
			}
		}(i, conf.Endpoints[i])
	}
	wait.Wait()
	DPrintf("🦖 All %d Connect", len(conf.Endpoints))
	return servers
}
