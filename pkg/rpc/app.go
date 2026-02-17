package rpc

import "github.com/vmkteam/zenrpc/v2"

type AppService struct {
	version string
	zenrpc.Service
}

func NewAppService(version string) *AppService {
	return &AppService{version: version}
}

// Version returns application version.
//
//zenrpc:return string
func (s AppService) Version() string {
	return s.version
}
