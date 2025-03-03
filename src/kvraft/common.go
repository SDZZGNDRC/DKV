package kvraft

// type Err string

const (
	ErrNotLeader       = "NotLeader"
	ErrKeyNotExist     = "KeyNotExist"
	ErrHandleOpTimeOut = "HandleOpTimeOut"
	ErrChanClose       = "ChanClose"
	ErrLeaderOutDated  = "LeaderOutDated"
	ERRRPCFailed       = "RPCFailed"
)
