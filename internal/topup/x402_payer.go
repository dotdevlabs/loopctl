package topup

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"

	secp "github.com/decred/dcrd/dcrec/secp256k1/v4"
	secp256ecdsa "github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/sha3"

	"github.com/dotdevlabs/ctlkit/pkg/ctxutil"
	"github.com/dotdevlabs/loopctl/internal/apiclient"
	"github.com/dotdevlabs/loopctl/internal/wallet"
)

// X402SignParams carries the EIP-3009 transferWithAuthorization parameters.
type X402SignParams struct {
	From        string // 0x-prefixed EVM address
	To          string // 0x-prefixed recipient
	Value       *big.Int
	ValidAfter  int64
	ValidBefore int64 // deadline from RailInfo
	Nonce       [32]byte
	ChainID     int64
	Token       string // USDC contract address
}

// X402Signature is the compact (v, r, s) ECDSA output.
type X402Signature struct {
	V uint8
	R [32]byte
	S [32]byte
}

// ArbitrumSigner signs EIP-3009 payloads for Arbitrum USDC.
type ArbitrumSigner interface {
	Address() string
	Sign(params X402SignParams) (X402Signature, error)
}

// X402Payer pays a 402 that advertises the "x402" rail using Arbitrum USDC.
type X402Payer struct {
	Endpoint string
	signer   ArbitrumSigner
}

// NewX402Payer constructs an X402Payer from an ArbitrumConfig.
// If the config is not configured, signer is nil and Configured() returns false.
func NewX402Payer(endpoint string, cfg wallet.ArbitrumConfig) *X402Payer {
	p := &X402Payer{Endpoint: endpoint}
	if cfg.Configured() {
		p.signer = newSecp256k1Signer(cfg.PrivateKey)
	}
	return p
}

func (p *X402Payer) RailName() string { return "x402" }
func (p *X402Payer) Configured() bool { return p.signer != nil && p.signer.Address() != "" }

func (p *X402Payer) Pay(ctx context.Context, product string, info apiclient.FundingInfo, out io.Writer) error {
	railInfo, err := findRail(info, product, "x402")
	if err != nil {
		return err
	}

	amountBig, ok := new(big.Int).SetString(railInfo.Amount, 10)
	if !ok {
		return fmt.Errorf("x402: invalid amount %q", railInfo.Amount)
	}

	nonceBytes, err := hexToBytes32(railInfo.Nonce)
	if err != nil {
		return fmt.Errorf("x402: invalid nonce: %w", err)
	}

	from := p.signer.Address()
	sig, err := p.signer.Sign(X402SignParams{
		From:        from,
		To:          railInfo.Recipient,
		Value:       amountBig,
		ValidAfter:  0,
		ValidBefore: railInfo.Deadline,
		Nonce:       nonceBytes,
		ChainID:     railInfo.ChainID,
		Token:       railInfo.Token,
	})
	if err != nil {
		return fmt.Errorf("x402: signing failed: %w", err)
	}

	payloadJSON, err := json.Marshal(struct {
		From        string `json:"from"`
		To          string `json:"to"`
		Value       string `json:"value"`
		ValidAfter  string `json:"validAfter"`
		ValidBefore string `json:"validBefore"`
		Nonce       string `json:"nonce"`
		V           uint8  `json:"v"`
		R           string `json:"r"`
		S           string `json:"s"`
	}{
		From:        from,
		To:          railInfo.Recipient,
		Value:       amountBig.String(),
		ValidAfter:  "0",
		ValidBefore: fmt.Sprintf("%d", railInfo.Deadline),
		Nonce:       "0x" + hex.EncodeToString(nonceBytes[:]),
		V:           sig.V,
		R:           "0x" + hex.EncodeToString(sig.R[:]),
		S:           "0x" + hex.EncodeToString(sig.S[:]),
	})
	if err != nil {
		return fmt.Errorf("x402: encoding payment header: %w", err)
	}

	paymentHeader := base64.StdEncoding.EncodeToString(payloadJSON)

	activeCtx := ctxutil.ActiveContextFrom(ctx)
	fullURL := strings.TrimRight(activeCtx.BaseURL, "/") + p.Endpoint //nolint:gosec // G107: URL from user config
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("x402: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+activeCtx.Token)
	req.Header.Set("X-PAYMENT", paymentHeader)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("x402: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("x402: payment request returned %d", resp.StatusCode)
	}

	jsonMode := ctxutil.GlobalFlagsFrom(ctx).JSON
	if jsonMode {
		return json.NewEncoder(out).Encode(struct {
			Rail    string `json:"rail"`
			Status  string `json:"status"`
			From    string `json:"from"`
			Product string `json:"product"`
		}{Rail: "x402", Status: "paid", From: from, Product: product})
	}
	_, err = fmt.Fprintf(out, "Payment sent via x402 from %s\n", from)
	return err
}

// findRail looks up the named rail for the given product in FundingInfo.
func findRail(info apiclient.FundingInfo, product, rail string) (apiclient.RailInfo, error) {
	for _, p := range info.Products {
		if p.Key == product {
			ri, ok := p.Rails[rail]
			if !ok {
				return apiclient.RailInfo{}, fmt.Errorf("product %q does not support %s rail", product, rail)
			}
			return ri, nil
		}
	}
	return apiclient.RailInfo{}, fmt.Errorf("product %q not found in funding info", product)
}

// hexToBytes32 decodes a hex string (with or without 0x prefix) into [32]byte.
func hexToBytes32(s string) ([32]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	b, err := hex.DecodeString(s)
	if err != nil {
		return [32]byte{}, err
	}
	if len(b) != 32 {
		return [32]byte{}, fmt.Errorf("expected 32 bytes, got %d", len(b))
	}
	var out [32]byte
	copy(out[:], b)
	return out, nil
}

// secp256k1Signer implements ArbitrumSigner using EIP-3009 / EIP-712.
type secp256k1Signer struct {
	privKey *secp.PrivateKey
	address string
}

func newSecp256k1Signer(hexKey string) *secp256k1Signer {
	b, err := hex.DecodeString(strings.TrimPrefix(hexKey, "0x"))
	if err != nil || len(b) != 32 {
		return &secp256k1Signer{}
	}
	priv := secp.PrivKeyFromBytes(b)
	addr := pubKeyToAddress(priv.PubKey())
	return &secp256k1Signer{privKey: priv, address: addr}
}

func (s *secp256k1Signer) Address() string { return s.address }

func (s *secp256k1Signer) Sign(p X402SignParams) (X402Signature, error) {
	if s.privKey == nil {
		return X402Signature{}, fmt.Errorf("no private key configured")
	}

	domainSep := eip712DomainSeparator(p.Token, p.ChainID)
	structHash := eip3009StructHash(p)
	digest := eip712Digest(domainSep, structHash)

	sig := secp256ecdsa.SignCompact(s.privKey, digest[:], false)
	// SignCompact returns [27 + recovery_id, r (32), s (32)] for uncompressed keys.
	// sig[0] is already in Ethereum's convention (27 or 28).
	if len(sig) != 65 {
		return X402Signature{}, fmt.Errorf("unexpected signature length %d", len(sig))
	}
	var r, sc [32]byte
	copy(r[:], sig[1:33])
	copy(sc[:], sig[33:65])
	return X402Signature{V: sig[0], R: r, S: sc}, nil
}

// pubKeyToAddress converts a secp256k1 public key to an Ethereum address.
func pubKeyToAddress(pub *secp.PublicKey) string {
	uncompressed := pub.SerializeUncompressed() // 65 bytes: 0x04 || X || Y
	h := keccak256(uncompressed[1:])            // hash X||Y (64 bytes)
	return "0x" + hex.EncodeToString(h[12:])    // last 20 bytes
}

// USDC on Arbitrum One: name="USD Coin", version="2".
// These constants match the on-chain USDC contract.
const (
	usdcName    = "USD Coin"
	usdcVersion = "2"
)

var (
	eip712DomainTypeHash = keccak256([]byte(
		"EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)",
	))
	transferWithAuthTypeHash = keccak256([]byte(
		"TransferWithAuthorization(address from,address to,uint256 value,uint256 validAfter,uint256 validBefore,bytes32 nonce)",
	))
)

func eip712DomainSeparator(tokenAddr string, chainID int64) [32]byte {
	nameHash := keccak256([]byte(usdcName))
	versionHash := keccak256([]byte(usdcVersion))

	addrBytes := evmAddrToBytes32(tokenAddr)
	chainIDBytes := uint256Bytes(big.NewInt(chainID))

	data := make([]byte, 0, 5*32)
	data = append(data, eip712DomainTypeHash[:]...)
	data = append(data, nameHash[:]...)
	data = append(data, versionHash[:]...)
	data = append(data, chainIDBytes[:]...)
	data = append(data, addrBytes[:]...)
	return keccak256(data)
}

func eip3009StructHash(p X402SignParams) [32]byte {
	fromBytes := evmAddrToBytes32(p.From)
	toBytes := evmAddrToBytes32(p.To)
	valueBytes := uint256Bytes(p.Value)
	validAfterBytes := uint256Bytes(big.NewInt(p.ValidAfter))
	validBeforeBytes := uint256Bytes(big.NewInt(p.ValidBefore))

	data := make([]byte, 0, 7*32)
	data = append(data, transferWithAuthTypeHash[:]...)
	data = append(data, fromBytes[:]...)
	data = append(data, toBytes[:]...)
	data = append(data, valueBytes[:]...)
	data = append(data, validAfterBytes[:]...)
	data = append(data, validBeforeBytes[:]...)
	data = append(data, p.Nonce[:]...)
	return keccak256(data)
}

func eip712Digest(domainSep, structHash [32]byte) [32]byte {
	data := make([]byte, 0, 2+32+32)
	data = append(data, 0x19, 0x01)
	data = append(data, domainSep[:]...)
	data = append(data, structHash[:]...)
	return keccak256(data)
}

// evmAddrToBytes32 right-aligns a 0x-prefixed EVM address into 32 bytes.
func evmAddrToBytes32(addr string) [32]byte {
	addr = strings.TrimPrefix(addr, "0x")
	b, _ := hex.DecodeString(addr)
	var out [32]byte
	copy(out[32-len(b):], b)
	return out
}

// uint256Bytes left-pads a big.Int into a big-endian 32-byte array.
func uint256Bytes(n *big.Int) [32]byte {
	var out [32]byte
	if n == nil {
		return out
	}
	nb := n.Bytes()
	copy(out[32-len(nb):], nb)
	return out
}

func keccak256(data []byte) [32]byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
