package core

import "errors"

// Sentinel errors downstream packages (registry, api) branch on.
var (
	// ErrProjectNotFound is returned when a project ID has no registered project.
	ErrProjectNotFound = errors.New("project not found")
	// ErrTicketNotFound is returned when a ticket ID does not exist in a project.
	ErrTicketNotFound = errors.New("ticket not found")
)
