package types

type OpGetReq struct {
	Key string
}

type OpGetResp struct {
	Value   string
	Err     string
	Success bool
}
