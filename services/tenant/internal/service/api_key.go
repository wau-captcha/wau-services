package service

import (
	"context"

	"tenant/internal/biz"

	v1 "common/proto/tenant/v1"

	"github.com/google/uuid"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ApiKeyService implements the tenant api key API.
type ApiKeyService struct {
	v1.UnimplementedApiKeyServiceServer

	keys *biz.ApiKeyUsecase
}

func NewApiKeyService(keys *biz.ApiKeyUsecase) *ApiKeyService { return &ApiKeyService{keys: keys} }

func (s *ApiKeyService) ListApiKeys(ctx context.Context, _ *v1.ListApiKeysRequest) (*v1.ApiKeySet, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	keys, err := s.keys.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	keySet := &v1.ApiKeySet{
		ApiKeys:    make([]*v1.ApiKey, 0, len(keys)),
		MaxApiKeys: int32(s.keys.MaxPerUser()),
	}
	for _, key := range keys {
		keySet.ApiKeys = append(keySet.ApiKeys, convertApiKey(key))
	}
	return keySet, nil
}

func (s *ApiKeyService) CreateApiKey(ctx context.Context, req *v1.CreateApiKeyRequest) (*v1.ApiKey, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	key, err := s.keys.Create(ctx, userID, req.GetName())
	if err != nil {
		return nil, err
	}
	return convertApiKey(key), nil
}

func (s *ApiKeyService) GetApiKey(ctx context.Context, req *v1.GetApiKeyRequest) (*v1.ApiKey, error) {
	userID, id, err := callerAndKey(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	key, err := s.keys.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return convertApiKey(key), nil
}

func (s *ApiKeyService) UpdateApiKey(ctx context.Context, req *v1.UpdateApiKeyRequest) (*v1.ApiKey, error) {
	userID, id, err := callerAndKey(ctx, req.GetApiKey().GetId())
	if err != nil {
		return nil, err
	}
	patch, err := patchFromMask(req.GetUpdateMask(), req.GetApiKey())
	if err != nil {
		return nil, err
	}
	key, err := s.keys.Update(ctx, userID, id, patch)
	if err != nil {
		return nil, err
	}
	return convertApiKey(key), nil
}

func (s *ApiKeyService) RotateSecretKey(ctx context.Context, req *v1.RotateSecretKeyRequest) (*v1.ApiKey, error) {
	userID, id, err := callerAndKey(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	key, err := s.keys.RotateSecret(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return convertApiKey(key), nil
}

func (s *ApiKeyService) RevokePreviousSecret(ctx context.Context, req *v1.RevokePreviousSecretRequest) (*v1.ApiKey, error) {
	userID, id, err := callerAndKey(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	key, err := s.keys.RevokePreviousSecret(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return convertApiKey(key), nil
}

func (s *ApiKeyService) SetAllowedOrigins(ctx context.Context, req *v1.SetAllowedOriginsRequest) (*v1.ApiKey, error) {
	userID, id, err := callerAndKey(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	origins := make([]biz.AllowedOrigin, 0, len(req.GetAllowedOrigins()))
	for _, origin := range req.GetAllowedOrigins() {
		origins = append(origins, biz.AllowedOrigin{IPAddress: origin.GetIpAddress(), Description: origin.GetDescription()})
	}
	key, err := s.keys.SetAllowedOrigins(ctx, userID, id, origins)
	if err != nil {
		return nil, err
	}
	return convertApiKey(key), nil
}

func (s *ApiKeyService) DeleteApiKey(ctx context.Context, req *v1.DeleteApiKeyRequest) (*emptypb.Empty, error) {
	userID, id, err := callerAndKey(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if err := s.keys.Delete(ctx, userID, id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func callerAndKey(ctx context.Context, rawID string) (userID, keyID uuid.UUID, err error) {
	userID, err = currentUser(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	keyID, err = parseKeyID(rawID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return userID, keyID, nil
}

// patchFromMask turns the field mask into the fields the usecase understands.
func patchFromMask(mask *fieldmaskpb.FieldMask, key *v1.ApiKey) (biz.ApiKeyPatch, error) {
	var patch biz.ApiKeyPatch
	for _, path := range mask.GetPaths() {
		switch path {
		case "name":
			name := key.GetName()
			patch.Name = &name
		case "is_active":
			isActive := key.GetIsActive()
			patch.IsActive = &isActive
		default:
			return patch, biz.InvalidField("update_mask", "update_mask accepts name and is_active")
		}
	}
	return patch, nil
}

func convertAllowedOrigins(origins []biz.AllowedOrigin) []*v1.AllowedOrigin {
	converted := make([]*v1.AllowedOrigin, 0, len(origins))
	for _, origin := range origins {
		converted = append(converted, &v1.AllowedOrigin{
			IpAddress:   origin.IPAddress,
			Description: origin.Description,
			CreateDate:  timestamppb.New(origin.CreateDate),
		})
	}
	return converted
}

func convertApiKey(key *biz.ApiKey) *v1.ApiKey {
	converted := &v1.ApiKey{
		Id:             key.ID.String(),
		Name:           key.Name,
		SiteKey:        key.SiteKey,
		SecretKey:      key.SecretKey,
		IsActive:       key.IsActive,
		CreateDate:     timestamppb.New(key.CreateDate),
		IsSuspended:    key.IsSuspended,
		SuspendReason:  key.SuspendReason,
		AllowedOrigins: convertAllowedOrigins(key.AllowedOrigins),
	}
	if key.PreviousSecretKey != "" {
		converted.PreviousSecretKey = key.PreviousSecretKey
		converted.PreviousSecretKeyExpiresAt = timestamppb.New(key.PreviousSecretExpiresAt)
	}
	return converted
}
