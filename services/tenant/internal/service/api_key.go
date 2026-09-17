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

	uc *biz.ApiKeyUsecase
}

func NewApiKeyService(uc *biz.ApiKeyUsecase) *ApiKeyService { return &ApiKeyService{uc: uc} }

func (s *ApiKeyService) ListApiKeys(ctx context.Context, _ *v1.ListApiKeysRequest) (*v1.ApiKeySet, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	keys, err := s.uc.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	set := &v1.ApiKeySet{
		ApiKeys:    make([]*v1.ApiKey, 0, len(keys)),
		MaxApiKeys: int32(s.uc.MaxPerUser()),
	}
	for _, key := range keys {
		set.ApiKeys = append(set.ApiKeys, convertApiKey(key))
	}
	return set, nil
}

func (s *ApiKeyService) CreateApiKey(ctx context.Context, req *v1.CreateApiKeyRequest) (*v1.ApiKey, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	key, err := s.uc.Create(ctx, userID, req.GetName())
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
	key, err := s.uc.Get(ctx, userID, id)
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
	key, err := s.uc.Update(ctx, userID, id, patch)
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
	key, err := s.uc.RotateSecret(ctx, userID, id)
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
	if err := s.uc.Delete(ctx, userID, id); err != nil {
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

func convertApiKey(key *biz.ApiKey) *v1.ApiKey {
	return &v1.ApiKey{
		Id:         key.ID.String(),
		Name:       key.Name,
		SiteKey:    key.SiteKey,
		SecretKey:  key.SecretKey,
		IsActive:   key.IsActive,
		CreateDate: timestamppb.New(key.CreateDate),
	}
}
