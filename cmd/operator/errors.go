package main

import "errors"

// ErrMultipleRoles is returned when a trigger has multiple roles
var ErrMultipleRoles = errors.New("can only add one role per trigger")

// ErrMultipleAddressRole is returned when a trigger has multiple address roles
var ErrMultipleAddressRole = errors.New("can only set one address role per trigger")
