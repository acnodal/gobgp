// Copyright (C) 2016 Nippon Telegraph and Telephone Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"

	"github.com/osrg/gobgp/v4/api"
	"github.com/stretchr/testify/assert"
)

type mockServer struct {
	api.UnimplementedGoBgpServiceServer
	grpcServer *grpc.Server
	lis        net.Listener
	nextResp   interface{}
	nextErr    error
	client     api.GoBgpServiceClient
}

func newMockServer() *mockServer {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	s := &mockServer{
		grpcServer: grpc.NewServer(),
		lis:        lis,
	}
	api.RegisterGoBgpServiceServer(s.grpcServer, s)
	go s.grpcServer.Serve(lis)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	s.client = api.NewGoBgpServiceClient(conn)
	return s
}

func (s *mockServer) stop() {
	s.grpcServer.Stop()
}

func (s *mockServer) setNextResponse(resp interface{}, err error) {
	s.nextResp = resp
	s.nextErr = err
}

func (s *mockServer) GetRedistribution(ctx context.Context, in *api.GetRedistributionRequest) (*api.GetRedistributionResponse, error) {
	return s.nextResp.(*api.GetRedistributionResponse), s.nextErr
}

func (s *mockServer) EnableRedistribution(ctx context.Context, in *api.EnableRedistributionRequest) (*api.EnableRedistributionResponse, error) {
	return s.nextResp.(*api.EnableRedistributionResponse), s.nextErr
}

func Test_ExtractReserved(t *testing.T) {
	assert := assert.New(t)
	args := strings.Split("10 rt 100:100 med 10 nexthop 10.0.0.1 aigp metric 10 local-pref 100", " ")
	keys := map[string]int{
		"rt":         paramList,
		"med":        paramSingle,
		"nexthop":    paramSingle,
		"aigp":       paramList,
		"local-pref": paramSingle,
	}
	m, _ := extractReserved(args, keys)
	assert.True(len(m["rt"]) == 1)
	assert.True(len(m["med"]) == 1)
	assert.True(len(m["nexthop"]) == 1)
	assert.True(len(m["aigp"]) == 2)
	assert.True(len(m["local-pref"]) == 1)
}