//go:build blockchain

// Package blockchain provides the Lethean blockchain as a Core service.
// Build tag "blockchain" enables blockchain commands in CoreApp builds.
//
// CoreIDE discovers c.Command() registrations from this package when
// the "blockchain" tag is active, enabling custom app composition.
//
//	go build -tags blockchain .
package blockchain
