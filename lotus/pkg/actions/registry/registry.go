package registry

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
)

type ActionFactory func(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error)

var Actions = map[string]ActionFactory{}

func GetAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	action, ok := Actions[key]
	if !ok {
		return nil, errors.NewMappingError("action not found")
	}
	return action(key, args, inputTypes...)
}
