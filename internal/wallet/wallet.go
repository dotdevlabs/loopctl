// Package wallet loads payment rail credentials from environment variables.
package wallet

import "os"

// ArbitrumConfig holds credentials for the x402 Arbitrum USDC rail.
type ArbitrumConfig struct {
	// PrivateKey is the 64-char hex secp256k1 private key (no 0x prefix).
	PrivateKey string
}

// Configured returns true when the minimum required fields are present.
func (c ArbitrumConfig) Configured() bool {
	return c.PrivateKey != ""
}

// LightningConfig holds credentials for the L402 Lightning rail.
type LightningConfig struct {
	// Host is the LND REST base URL, e.g. https://localhost:8080.
	Host string
	// MacaroonHex is the hex-encoded admin/invoice macaroon.
	MacaroonHex string
	// TLSSkipVerify disables TLS certificate verification (dev only).
	TLSSkipVerify bool
}

// Configured returns true when the minimum required fields are present.
func (c LightningConfig) Configured() bool {
	return c.Host != "" && c.MacaroonHex != ""
}

// WalletConfig aggregates all rail-specific credentials.
type WalletConfig struct {
	Arbitrum  ArbitrumConfig
	Lightning LightningConfig
}

// Load reads wallet credentials from environment variables.
// Never logs or echoes any secret values.
func Load() *WalletConfig {
	return &WalletConfig{
		Arbitrum: ArbitrumConfig{
			PrivateKey: os.Getenv("LOOPCTL_ARBITRUM_PRIVATE_KEY"),
		},
		Lightning: LightningConfig{
			Host:          os.Getenv("LOOPCTL_LN_HOST"),
			MacaroonHex:   os.Getenv("LOOPCTL_LN_MACAROON_HEX"),
			TLSSkipVerify: os.Getenv("LOOPCTL_LN_TLS_SKIP_VERIFY") == "true",
		},
	}
}
