package core

import "errors"

var (
	// ErrInternal raised on any unexpected error due to internal conditions.
	// Most likely due to the disk failures.
	ErrInternal = errors.New("internal")
	// ErrMalformed raised if transaction cannot be decoded properly.
	ErrMalformed = errors.New("malformed tx")
	// ErrInvalidNonce raised due to the expected nonce mismatch.
	ErrInvalidNonce = errors.New("invalid nonce")
	// ErrNoBalance raised if transaction run out of balance during execution.
	ErrNoBalance = errors.New("no balance")
	// ErrMaxGas raised if tx consumed over MaxGas value.
	ErrMaxGas = errors.New("max gas")
	// ErrMaxSpend raised if tx transferred over MaxSpend value.
	ErrMaxSpend = errors.New("max spend")
	// ErrSpawned raised if account already spawned.
	ErrSpawned = errors.New("account already spawned")
	// ErrNotSpawned raised if account is not spawned.
	ErrNotSpawned = errors.New("account is not spawned")
	// ErrAuth raised if principal wasn't authorized by the target account for foreign call.
	ErrAuth = errors.New("failed authorization")
)
