package main

import (
	"challenge/internal/biz"
	"challenge/internal/conf"
	"challenge/internal/data"
	"challenge/internal/server"
	"challenge/internal/service"
	"log/slog"

	"github.com/go-kratos/kratos/v3"

	_ "go.uber.org/automaxprocs"
)

func wireApp(confServer *conf.Server, confData *conf.Data, challenge *conf.Challenge, logger *slog.Logger) (*kratos.App, func(), error) {
	dataData, cleanup, err := data.NewData(confData)
	if err != nil {
		return nil, nil, err
	}
	siteRepo := data.NewSiteRepo(dataData)
	challengeRepo := data.NewChallengeRepo(dataData)
	challengeUsecase, err := biz.NewChallengeUsecase(siteRepo, challengeRepo, challenge)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	challengeService := service.NewChallengeService(challengeUsecase)
	grpcServer := server.NewGRPCServer(confServer, challengeService)
	httpServer := server.NewHTTPServer(confServer, challengeService)
	app := newApp(logger, grpcServer, httpServer)
	return app, func() {
		cleanup()
	}, nil
}
