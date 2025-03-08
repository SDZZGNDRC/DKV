package types

type OpPutReq struct {
	Key   string
	Value string
}

type OpPutResp struct {
	Success bool
	Err     string
}
