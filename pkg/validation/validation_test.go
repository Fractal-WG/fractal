package validation

import (
	"strings"
	"testing"

	"dogecoin.org/fractal-engine/pkg/doge"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{"Valid mainnet address", "D7P2jVEK6JGiGepUGTAHqELKK8QJ8GCahZ", false},
		{"Valid testnet address", "nTQNmAFNcpZUMoLCVh7GU8Jd5HaYLrDr7b", false},
		{"Empty address", "", true},
		{"Too short", "D123", true},
		{"Too long", strings.Repeat("D", 70), true},
		{"Invalid format", "InvalidAddress123", true},
		{"Invalid characters", "D7P2jVEK6JGiGepUGTAHqELKK8QJ8GCah!", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddress(tt.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHash(t *testing.T) {
	tests := []struct {
		name    string
		hash    string
		wantErr bool
	}{
		{"Valid hash", "a1b2c3d4e5f6789012345678901234567890123456789012345678901234abcd", false},
		{"Empty hash", "", true},
		{"Too short", "a1b2c3d4", true},
		{"Too long", "a1b2c3d4e5f6789012345678901234567890123456789012345678901234abcdef", true},
		{"Invalid characters", "g1b2c3d4e5f6789012345678901234567890123456789012345678901234abcd", true},
		{"Uppercase valid", "A1B2C3D4E5F6789012345678901234567890123456789012345678901234ABCD", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHash(tt.hash)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHash() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePublicKey(t *testing.T) {
	tests := []struct {
		name    string
		pubKey  string
		wantErr bool
	}{
		{"Valid compressed public key", "02e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", false},
		{"Valid compressed public key 03", "03e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", false},
		{"Empty public key", "", true},
		{"Too short", "02e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b8", true},
		{"Too long", "02e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85555", true},
		{"Invalid characters", "02g3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePublicKey(tt.pubKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePublicKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAddressPublicKeyMatch(t *testing.T) {
	_, pubKey, address, err := doge.GenerateDogecoinKeypair(doge.PrefixMainnet)
	if err != nil {
		t.Fatalf("GenerateDogecoinKeypair: %v", err)
	}

	_, badPub, _, err := doge.GenerateDogecoinKeypair(doge.PrefixMainnet)
	if err != nil {
		t.Fatalf("GenerateDogecoinKeypair: %v", err)
	}

	tests := []struct {
		name    string
		addr    string
		pubKey  string
		wantErr bool
	}{
		{"Matching mainnet address", address, pubKey, false},
		{"Mismatched mainnet address", address, badPub, true},
		{"Invalid address", "not-an-address", pubKey, true},
		{"Invalid pubkey", address, "1234", true},
		{"P2SH address not supported", "A7P2jVEK6JGiGepUGTAHqELKK8QJ8GCahZ", pubKey, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddressPublicKeyMatch(tt.addr, tt.pubKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAddressPublicKeyMatch() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTitle(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		wantErr bool
	}{
		{"Valid title", "My Test Token", false},
		{"Empty title", "", true},
		{"Max length title", strings.Repeat("a", MaxTitleLength), false},
		{"Too long title", strings.Repeat("a", MaxTitleLength+1), true},
		{"Invalid characters", "My Token <script>", true},
		{"Special chars allowed", "Token-2024_v1.0!", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTitle(tt.title)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTitle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateQuantity(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		quantity int
		wantErr  bool
	}{
		{"Valid quantity", "amount", 1000, false},
		{"Zero quantity", "amount", 0, true},
		{"Negative quantity", "amount", -100, true},
		{"Max quantity", "amount", MaxQuantity, false},
		{"Over max quantity", "amount", MaxQuantity+1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQuantity(tt.field, tt.quantity)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateQuantity() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTags(t *testing.T) {
	tests := []struct {
		name    string
		tags    []string
		wantErr bool
	}{
		{"Valid tags", []string{"token", "test", "demo"}, false},
		{"Empty tags", []string{}, false},
		{"Too many tags", make([]string, MaxTagCount+1), true},
		{"Empty tag", []string{"valid", "", "tags"}, true},
		{"Too long tag", []string{strings.Repeat("a", MaxTagLength+1)}, true},
		{"Invalid characters", []string{"valid", "tag<script>", "safe"}, true},
	}

	// Fill the "too many tags" test with valid tags
	tests[2].tags = make([]string, MaxTagCount+1)
	for i := range tests[2].tags {
		tests[2].tags[i] = "tag"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTags(tt.tags)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTags() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeQueryParam(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Normal string", "test123", "test123"},
		{"With newlines", "test\nvalue", "testvalue"},
		{"With tabs", "test\tvalue", "testvalue"},
		{"With null bytes", "test\x00value", "testvalue"},
		{"With spaces", "  test value  ", "test value"},
		{"Too long", strings.Repeat("a", 150), strings.Repeat("a", 100)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeQueryParam(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeQueryParam() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateProtobufActionType(t *testing.T) {
	tests := []struct {
		name       string
		actionType uint32
		wantErr    bool
	}{
		{"Valid action type 1", 1, false},
		{"Valid action type 2", 2, false},
		{"Valid action type 3", 3, false},
		{"Invalid action type 0", 0, true},
		{"Invalid action type 99", 99, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProtobufActionType(tt.actionType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProtobufActionType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
