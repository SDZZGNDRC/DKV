package types

type OpAppendReq struct {
	Key   string
	Value string
}

type OpAppendResp struct {
	Success bool
	Err     string
}
