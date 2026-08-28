package service

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/meddion/live-kit/api"
	"github.com/meddion/live-kit/auth"
)

type Store interface {
	Permissions(c context.Context, user string) ([]api.Permission, error)
}

type PermissionCheckerDB struct {
	db Store
}

func NewPermissionChecker(db Store) *PermissionCheckerDB {
	return &PermissionCheckerDB{db: db}
}

func (this *PermissionCheckerDB) Check(c context.Context, targetPerms ...api.Permission) (bool, error) {
	identity, ok := auth.IdentityFromContext(c)
	if !ok {
		return false, fmt.Errorf("no identity in context: %w", api.ErrNotAuthorized)
	}

	userPerms, err := this.db.Permissions(c, identity)
	if err != nil {
		return false, fmt.Errorf("failed to get permissions for user %q: %w", identity, err)
	}

	if slices.Contains(userPerms, api.PermGodAlmighty) {
		slog.Debug("user has GodAlmighty permission, granting all permissions", "user", identity)
		return true, nil
	}

	for _, required := range targetPerms {
		if !slices.Contains(userPerms, required) {
			slog.Debug("user does not have required permission", "user", identity, "required", required, "user_perms", userPerms)
			return false, nil
		}
	}

	return true, nil
}
