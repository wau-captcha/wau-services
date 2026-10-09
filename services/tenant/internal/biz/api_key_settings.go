package biz

import (
	"context"
	"slices"

	"github.com/google/uuid"
)

// Update renames, activates or pauses a key, or turns require_behavior on or off. Each change
// that takes effect goes into the activity log.
func (uc *ApiKeyUsecase) Update(ctx context.Context, userID, id uuid.UUID, patch ApiKeyPatch) (*ApiKey, error) {
	if patch.Name == nil && patch.IsActive == nil && patch.RequireBehavior == nil {
		return nil, InvalidField("update_mask", "nothing to update")
	}
	if patch.Name != nil {
		name, err := normalizeKeyName(*patch.Name)
		if err != nil {
			return nil, err
		}
		patch.Name = &name
	}
	before, err := uc.keys.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	key, err := uc.keys.Update(ctx, userID, id, patch)
	if err != nil {
		return nil, err
	}
	actor := Actor{ActorUser, userID}
	if key.Name != before.Name {
		recordEvent(ctx, uc.events, id, actor, EventRenamed, map[string]any{"from": before.Name, "to": key.Name})
	}
	if key.IsActive != before.IsActive {
		action := EventPaused
		if key.IsActive {
			action = EventActivated
		}
		recordEvent(ctx, uc.events, id, actor, action, nil)
	}
	if key.RequireBehavior != before.RequireBehavior {
		recordEvent(ctx, uc.events, id, actor, EventRequireBehaviorChanged, map[string]any{"require_behavior": key.RequireBehavior})
	}
	uc.markFlagged(key)
	return key, nil
}

// RotateSecret issues a new secret key and keeps the replaced one working for the grace
// period, so a site can roll the new one out without downtime. A reset inside that window
// overwrites the previous key: only one is ever kept.
func (uc *ApiKeyUsecase) RotateSecret(ctx context.Context, userID, id uuid.UUID) (*ApiKey, error) {
	secretKey, err := newKeyValue(secretKeyPrefix)
	if err != nil {
		return nil, err
	}
	key, err := uc.keys.RotateSecret(ctx, userID, id, secretKey, uc.previousTTL)
	if err != nil {
		return nil, err
	}
	recordEvent(ctx, uc.events, id, Actor{ActorUser, userID}, EventSecretRotated, nil)
	uc.markFlagged(key)
	return key, nil
}

// RevokePreviousSecret stops the replaced secret key before its grace period is over.
func (uc *ApiKeyUsecase) RevokePreviousSecret(ctx context.Context, userID, id uuid.UUID) (*ApiKey, error) {
	before, err := uc.keys.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	key, err := uc.keys.RevokePreviousSecret(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if before.PreviousSecretKey != "" {
		recordEvent(ctx, uc.events, id, Actor{ActorUser, userID}, EventPreviousSecretRevoked, nil)
	}
	uc.markFlagged(key)
	return key, nil
}

// SetAllowedOrigins replaces the addresses allowed to use the key. An empty list lets
// every address use it again.
func (uc *ApiKeyUsecase) SetAllowedOrigins(ctx context.Context, userID, id uuid.UUID, origins []AllowedOrigin) (*ApiKey, error) {
	normalized, err := NormalizeOrigins(origins)
	if err != nil {
		return nil, err
	}
	// Reading the key first keeps another user's key indistinguishable from a missing one.
	key, err := uc.keys.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	before, err := uc.keys.AllowedOrigins(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	key.AllowedOrigins, err = uc.keys.SetAllowedOrigins(ctx, userID, id, normalized)
	if err != nil {
		return nil, err
	}
	if after := addresses(key.AllowedOrigins); !sameSet(addresses(before), after) {
		recordEvent(ctx, uc.events, id, Actor{ActorUser, userID}, EventAllowedIPsChanged, map[string]any{"ip_addresses": after})
	}
	uc.markFlagged(key)
	return key, nil
}

// SetAllowedDomains replaces the websites allowed to use the key. An empty list lets any
// website use it again.
func (uc *ApiKeyUsecase) SetAllowedDomains(ctx context.Context, userID, id uuid.UUID, rawDomains []string) (*ApiKey, error) {
	domains, err := NormalizeDomains(rawDomains)
	if err != nil {
		return nil, err
	}
	key, err := uc.keys.Get(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	before, err := uc.keys.AllowedDomains(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	key.AllowedDomains, err = uc.keys.SetAllowedDomains(ctx, userID, id, domains)
	if err != nil {
		return nil, err
	}
	if after := domainNames(key.AllowedDomains); !sameSet(domainNames(before), after) {
		recordEvent(ctx, uc.events, id, Actor{ActorUser, userID}, EventAllowedDomainsChanged, map[string]any{"domains": after})
	}
	uc.markFlagged(key)
	return key, nil
}

func domainNames(domains []AllowedDomain) []string {
	names := make([]string, len(domains))
	for i, domain := range domains {
		names[i] = domain.Domain
	}
	return names
}

// sameSet reports whether two lists hold the same entries, in any order.
func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
