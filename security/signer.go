package security

// Signer defines an interface for signing data and accessing the public key.
type Signer interface {
	Sign(data []byte) ([]byte, error)
	PublicKey() []byte // Returns the public key associated with the signer
	Address() string   // Returns a string representation of the address/ID (e.g., hex-encoded pubkey)
}
