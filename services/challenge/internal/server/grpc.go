package server

import (
	v1 "challenge/api/challenge/v1"
	"challenge/internal/conf"
	"challenge/internal/service"

	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/transport/grpc"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(serverConf *conf.Server, challenge *service.ChallengeService) *grpc.Server {
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
			// verify passes on what CheckAnswer refuses, so its errors must be fit for clients too.
			sanitizeErrors(),
		),
	}
	if serverConf.Grpc.Network != "" {
		opts = append(opts, grpc.Network(serverConf.Grpc.Network))
	}
	if serverConf.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(serverConf.Grpc.Addr))
	}
	if serverConf.Grpc.Timeout != nil {
		opts = append(opts, grpc.Timeout(serverConf.Grpc.Timeout.AsDuration()))
	}
	grpcServer := grpc.NewServer(opts...)
	v1.RegisterChallengeServiceServer(grpcServer, challenge)
	return grpcServer
}
