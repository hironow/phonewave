// Package domain contains domain types, events, commands, policies, and
// aggregates. Types may reference context.Context in interface definitions
// but implementations must not perform I/O directly.
// I/O operations belong in the session layer; orchestration belongs in usecase.
package domain
