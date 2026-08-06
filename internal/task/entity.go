package task

import (
	"errors"

	"github.com/google/uuid"
)

var (
	// ErrEntityIdentityMissing identifies a task without exactly one valid entity
	// for the requested relation.
	ErrEntityIdentityMissing = errors.New("task entity identity is missing")
)

// EntityIDByRelation returns exactly one UUID entity for a task relation.
// Matching is deliberately strict: a successful task with no matching entity,
// an invalid identifier, or duplicates is not safe for Terraform identity.
func EntityIDByRelation(snapshot Snapshot, relation string) (string, error) {
	if relation == "" {
		return "", ErrEntityIdentityMissing
	}
	var entityID string
	for _, entity := range snapshot.EntitiesAffected {
		if entity.Rel != relation {
			continue
		}
		parsed, err := uuid.Parse(entity.ExtID)
		if err != nil || entityID != "" {
			return "", ErrEntityIdentityMissing
		}
		entityID = parsed.String()
	}
	if entityID == "" {
		return "", ErrEntityIdentityMissing
	}
	return entityID, nil
}
