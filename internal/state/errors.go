package state

import "errors"

var (
	ErrKeyNotFound    = errors.New("blackboard key not found")
	ErrVersionConflict = errors.New("workflow state version conflict: concurrent write detected")
)
