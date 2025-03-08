package types

type SysStatus struct {
	Role      string
	ServerId  int
	Term      int
	VotedFor  int
	Timestamp int64
}
