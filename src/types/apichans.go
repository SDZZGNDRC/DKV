package types

type APIChans struct {
	GetSysStatusReqChan  chan struct{}
	GetSysStatusRespChan chan *SysStatus

	OpGetReqChan  chan *OpGetReq
	OpGetRespChan chan *OpGetResp

	OpAppendReqChan  chan *OpAppendReq
	OpAppendRespChan chan *OpAppendResp

	OpPutReqChan  chan *OpPutReq
	OpPutRespChan chan *OpPutResp
}
