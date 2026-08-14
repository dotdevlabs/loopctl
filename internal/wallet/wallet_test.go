package wallet

import "testing"

func TestLoadEmpty(t *testing.T) {
	wc := Load()
	if wc.Arbitrum.Configured() {
		t.Error("expected ArbitrumConfig.Configured() == false with no env vars")
	}
	if wc.Lightning.Configured() {
		t.Error("expected LightningConfig.Configured() == false with no env vars")
	}
}

func TestLoadArbitrumFromEnv(t *testing.T) {
	t.Setenv("LOOPCTL_ARBITRUM_PRIVATE_KEY", "deadbeefdeadbeef")
	wc := Load()
	if !wc.Arbitrum.Configured() {
		t.Fatal("expected ArbitrumConfig.Configured() == true")
	}
	if wc.Arbitrum.PrivateKey != "deadbeefdeadbeef" {
		t.Errorf("unexpected private key: %q", wc.Arbitrum.PrivateKey)
	}
}

func TestLoadLightningFromEnv(t *testing.T) {
	t.Setenv("LOOPCTL_LN_HOST", "https://localhost:8080")
	t.Setenv("LOOPCTL_LN_MACAROON_HEX", "cafebabe")
	t.Setenv("LOOPCTL_LN_TLS_SKIP_VERIFY", "true")
	wc := Load()
	if !wc.Lightning.Configured() {
		t.Fatal("expected LightningConfig.Configured() == true")
	}
	if wc.Lightning.Host != "https://localhost:8080" {
		t.Errorf("unexpected host: %q", wc.Lightning.Host)
	}
	if wc.Lightning.MacaroonHex != "cafebabe" {
		t.Errorf("unexpected macaroon: %q", wc.Lightning.MacaroonHex)
	}
	if !wc.Lightning.TLSSkipVerify {
		t.Error("expected TLSSkipVerify == true")
	}
}

func TestLoadLightningMissingMacaroon(t *testing.T) {
	t.Setenv("LOOPCTL_LN_HOST", "https://localhost:8080")
	wc := Load()
	if wc.Lightning.Configured() {
		t.Error("expected LightningConfig.Configured() == false without macaroon")
	}
}

func TestLoadLightningMissingHost(t *testing.T) {
	t.Setenv("LOOPCTL_LN_MACAROON_HEX", "cafebabe")
	wc := Load()
	if wc.Lightning.Configured() {
		t.Error("expected LightningConfig.Configured() == false without host")
	}
}

func TestLoadTLSSkipVerifyFalseByDefault(t *testing.T) {
	t.Setenv("LOOPCTL_LN_HOST", "https://localhost:8080")
	t.Setenv("LOOPCTL_LN_MACAROON_HEX", "cafebabe")
	wc := Load()
	if wc.Lightning.TLSSkipVerify {
		t.Error("expected TLSSkipVerify == false by default")
	}
}
