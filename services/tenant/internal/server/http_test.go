package server

import (
	"context"
	"testing"

	v1 "tenant/api/tenant/v1"
)

func TestUserOperation(t *testing.T) {
	tests := []struct {
		operation string
		needsUser bool
	}{
		{v1.OperationCatalogServiceListPublicPlans, false},
		{v1.OperationAuthServiceSignInWithGoogle, false},
		{v1.OperationAuthServiceSignInAdminWithGoogle, false},
		{v1.OperationAdminServiceGetStats, false},
		{v1.OperationCatalogServiceListChallengeTypes, true},
		{v1.OperationApiKeyServiceListApiKeys, true},
		{v1.OperationApiKeyServiceSetAllowedDomains, true},
		{v1.OperationApiKeyServiceCreateAppeal, true},
		{v1.OperationApiKeyServiceListApiKeyEvents, true},
		// Administrator tokens only, checked by the admin middleware.
		{v1.OperationAdminServiceListAppeals, false},
		{v1.OperationAdminServiceDecideAppeal, false},
	}
	for _, test := range tests {
		if got := userOperation(context.Background(), test.operation); got != test.needsUser {
			t.Errorf("userOperation(%s) = %v, want %v", test.operation, got, test.needsUser)
		}
	}
}
