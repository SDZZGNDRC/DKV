package types

type APIChans struct {
	GetSysStatusReqChan  chan struct{}
	GetSysStatusRespChan chan *SysStatus

	OpGetReqChan  chan *string
	OpGetRespChan chan *string

	OpAppendReqChan  chan *OpAppendReq
	OpAppendRespChan chan *OpAppendResp

	OpPutReqChan  chan *OpPutReq
	OpPutRespChan chan *OpPutResp
}
