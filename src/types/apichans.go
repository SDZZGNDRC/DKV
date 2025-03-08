package types

type APIChans struct {
	GetSysStatusReqChan  chan struct{}
	GetSysStatusRespChan chan *SysStatus
}
