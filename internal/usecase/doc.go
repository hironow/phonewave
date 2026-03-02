// Package usecase orchestrates domain aggregates and session I/O adapters.
// It validates COMMAND objects, delegates to aggregates for event production,
// and dispatches events through the PolicyEngine.
package usecase
