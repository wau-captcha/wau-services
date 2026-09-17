package auth

import (
	"tenant/internal/biz"

	"github.com/google/wire"
)

// ProviderSet is auth providers.
var ProviderSet = wire.NewSet(
	NewJWT,
	NewBcrypt,
	wire.Bind(new(biz.TokenIssuer), new(*JWT)),
	wire.Bind(new(biz.PasswordHasher), new(*Bcrypt)),
)
