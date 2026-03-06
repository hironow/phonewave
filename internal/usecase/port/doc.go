// Package port defines context-aware interface contracts and trivial default
// implementations (null objects) for the port-adapter pattern.
// Concrete I/O implementations live in session and platform layers.
// Port may only import domain (+ stdlib such as context, errors).
// No imports of upper internal layers (cmd, usecase root, session, eventsource, platform).
//
// This package lives under usecase/ because it represents the Output Port
// boundary of the usecase layer (hexagonal architecture).
package port
