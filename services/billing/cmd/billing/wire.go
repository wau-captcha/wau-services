//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"log/slog"

	"billing/internal/auth"
	"billing/internal/biz"
	"billing/internal/conf"
	"billing/internal/data"
	"billing/internal/server"
	"billing/internal/service"

	"github.com/go-kratos/kratos/v3"
	"github.com/google/wire"
)

// wireApp init kratos application.
func wireApp(*conf.Server, *conf.Data, *conf.Auth, *conf.Sepay, *conf.Billing, *slog.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, service.ProviderSet, auth.ProviderSet, newApp))
}
