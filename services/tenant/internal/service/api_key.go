package service

import (
	"context"

	v1 "tenant/api/tenant/v1"
	"tenant/internal/biz"

	"github.com/google/uuid"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
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
	key, err := s.keys.Create(ctx, userID, req.GetName(), req.GetAllowedDomains())
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

func (s *ApiKeyService) SetAllowedDomains(ctx context.Context, req *v1.SetAllowedDomainsRequest) (*v1.ApiKey, error) {
	userID, id, err := callerAndKey(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	domains := make([]string, 0, len(req.GetAllowedDomains()))
	for _, domain := range req.GetAllowedDomains() {
		domains = append(domains, domain.GetDomain())
	}
	key, err := s.keys.SetAllowedDomains(ctx, userID, id, domains)
	if err != nil {
		return nil, err
	}
	return convertApiKey(key), nil
}

func (s *ApiKeyService) CreateAppeal(ctx context.Context, req *v1.CreateAppealRequest) (*v1.ApiKeyAppeal, error) {
	userID, id, err := callerAndKey(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	appeal, err := s.keys.CreateAppeal(ctx, userID, id, req.GetMessage())
	if err != nil {
		return nil, err
	}
	return convertAppeal(appeal), nil
}

func (s *ApiKeyService) ListApiKeyEvents(ctx context.Context, req *v1.ListApiKeyEventsRequest) (*v1.ApiKeyEventSet, error) {
	userID, id, err := callerAndKey(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	events, err := s.keys.Events(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	reply := &v1.ApiKeyEventSet{Events: make([]*v1.ApiKeyEvent, 0, len(events))}
	for _, event := range events {
		converted, err := convertEvent(event)
		if err != nil {
			return nil, err
		}
		reply.Events = append(reply.Events, converted)
	}
	return reply, nil
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
		case "require_behavior":
			requireBehavior := key.GetRequireBehavior()
			patch.RequireBehavior = &requireBehavior
		default:
			return patch, biz.InvalidField("update_mask", "update_mask accepts name, is_active and require_behavior")
		}
	}
	return patch, nil
}
