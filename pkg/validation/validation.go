package validation

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"dogecoin.org/fractal-engine/pkg/doge"
)

const (
	// String length limits
	MaxTitleLength       = 100
	MaxDescriptionLength = 1000
	MaxFeedURLLength     = 500
	MaxTagLength         = 50
	MaxTagCount          = 20
	MaxMetadataSize      = 10000 // JSON bytes

	// Numeric limits
	MaxQuantity      = 1000000000 // 1 billion
	MaxPrice         = 1000000000 // 1 billion (in smallest unit)
	MaxFractionCount = 1000000000 // 1 billion

	// Hash and address formats
	HashLength       = 64 // SHA256 hex length
	MinAddressLength = 26
	MaxAddressLength = 62
)

var (
	// Dogecoin address regex patterns
	// Mainnet: P2PKH (D) or P2SH (A)
	mainnetRegex = regexp.MustCompile(`^(D|A)[1-9A-HJ-NP-Za-km-z]{25,34}$`)

	// Testnet/Regtest: P2PKH (m or n) or P2SH (2)
	testnetRegex = regexp.MustCompile(`^([mn2])[1-9A-HJ-NP-Za-km-z]{25,34}$`)

	// Hash format (64 character hex string)
	hashRegex = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

	// Public key format (66 character hex string - compressed)
	publicKeyRegex = regexp.MustCompile(`^[a-fA-F0-9]{66}$`)

	// Safe string pattern (alphanumeric, spaces, basic punctuation)
	safeStringRegex = regexp.MustCompile(`^[a-zA-Z0-9\s\-_.,!?()]+$`)
)

// ValidateAddress validates Dogecoin address format
func ValidateAddress(address string) error {
	if address == "" {
		return fmt.Errorf("address is required")
	}

	if len(address) < MinAddressLength || len(address) > MaxAddressLength {
		return fmt.Errorf("address length must be between %d and %d characters", MinAddressLength, MaxAddressLength)
	}

	// Check mainnet format
	if mainnetRegex.MatchString(address) {
		return nil
	}

	// Check testnet format
	if testnetRegex.MatchString(address) {
		return nil
	}

	return fmt.Errorf("invalid Dogecoin address format")
}

// ValidateHash validates SHA256 hash format
func ValidateHash(hash string) error {
	if hash == "" {
		return fmt.Errorf("hash is required")
	}

	if len(hash) != HashLength {
		return fmt.Errorf("hash must be exactly %d characters", HashLength)
	}

	if !hashRegex.MatchString(hash) {
		return fmt.Errorf("hash must be a valid hexadecimal string")
	}

	return nil
}

// ValidatePublicKey validates public key format
func ValidatePublicKey(pubKey string) error {
	if pubKey == "" {
		return fmt.Errorf("public key is required")
	}

	if len(pubKey) != 66 {
		return fmt.Errorf("public key must be exactly 66 characters (compressed format)")
	}

	if !publicKeyRegex.MatchString(pubKey) {
		return fmt.Errorf("public key must be a valid hexadecimal string")
	}

	// Verify it's a valid hex string
	_, err := hex.DecodeString(pubKey)
	if err != nil {
		return fmt.Errorf("public key is not valid hexadecimal: %w", err)
	}

	return nil
}

// ValidateAddressPublicKeyMatch checks that a P2PKH address matches the provided compressed public key.
func ValidateAddressPublicKeyMatch(address, pubKey string) error {
	if err := ValidateAddress(address); err != nil {
		return err
	}

	if err := ValidatePublicKey(pubKey); err != nil {
		return err
	}

	switch address[0] {
	case 'D':
		expected, err := doge.PublicKeyToDogeAddress(pubKey, doge.PrefixMainnet)
		if err != nil {
			return err
		}
		if address != expected {
			return fmt.Errorf("address does not match public key")
		}
		return nil
	case 'm', 'n':
		expectedTestnet, err := doge.PublicKeyToDogeAddress(pubKey, doge.PrefixTestnet)
		if err != nil {
			return err
		}
		expectedRegtest, err := doge.PublicKeyToDogeAddress(pubKey, doge.PrefixRegtest)
		if err != nil {
			return err
		}
		if address != expectedTestnet && address != expectedRegtest {
			return fmt.Errorf("address does not match public key")
		}
		return nil
	case 'A', '2':
		return fmt.Errorf("address must be P2PKH to match public key")
	default:
		return fmt.Errorf("unsupported address prefix")
	}
}

// ValidateStringLength validates string length with custom limits
func ValidateStringLength(field, value string, maxLength int) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s contains invalid UTF-8 characters", field)
	}

	if utf8.RuneCountInString(value) > maxLength {
		return fmt.Errorf("%s exceeds maximum length of %d characters", field, maxLength)
	}

	return nil
}

// ValidateTitle validates mint/offer title
func ValidateTitle(title string) error {
	if title == "" {
		return fmt.Errorf("title is required")
	}

	if err := ValidateStringLength("title", title, MaxTitleLength); err != nil {
		return err
	}

	// Additional safety check for potentially malicious content
	if !safeStringRegex.MatchString(title) {
		return fmt.Errorf("title contains invalid characters")
	}

	return nil
}

// ValidateDescription validates description field
func ValidateDescription(description string) error {
	if description == "" {
		return fmt.Errorf("description is required")
	}

	if err := ValidateStringLength("description", description, MaxDescriptionLength); err != nil {
		return err
	}

	return nil
}

// ValidateFeedURL validates feed URL
func ValidateFeedURL(feedURL string) error {
	if feedURL == "" {
		return nil
	}

	if err := ValidateStringLength("feed_url", feedURL, MaxFeedURLLength); err != nil {
		return err
	}

	// Basic URL format check
	if !strings.HasPrefix(feedURL, "http://") && !strings.HasPrefix(feedURL, "https://") {
		return fmt.Errorf("feed_url must be a valid HTTP/HTTPS URL")
	}

	return nil
}

// ValidateQuantity validates quantity values
func ValidateQuantity(field string, quantity int) error {
	if quantity <= 0 {
		return fmt.Errorf("%s must be greater than 0", field)
	}

	if quantity > MaxQuantity {
		return fmt.Errorf("%s exceeds maximum value of %d", field, MaxQuantity)
	}

	return nil
}

// ValidatePrice validates price values
func ValidatePrice(field string, price int) error {
	if price <= 0 {
		return fmt.Errorf("%s must be greater than 0", field)
	}

	if price > MaxPrice {
		return fmt.Errorf("%s exceeds maximum value of %d", field, MaxPrice)
	}

	return nil
}

// ValidateTags validates tag array
func ValidateTags(tags []string) error {
	if len(tags) > MaxTagCount {
		return fmt.Errorf("too many tags (maximum %d allowed)", MaxTagCount)
	}

	for i, tag := range tags {
		if tag == "" {
			return fmt.Errorf("tag %d is empty", i+1)
		}

		if err := ValidateStringLength(fmt.Sprintf("tag %d", i+1), tag, MaxTagLength); err != nil {
			return err
		}

		if !safeStringRegex.MatchString(tag) {
			return fmt.Errorf("tag %d contains invalid characters", i+1)
		}
	}

	return nil
}

// ValidateMetadataSize validates metadata JSON size
func ValidateMetadataSize(field string, data []byte) error {
	if len(data) > MaxMetadataSize {
		return fmt.Errorf("%s exceeds maximum size of %d bytes", field, MaxMetadataSize)
	}

	return nil
}

// SanitizeQueryParam sanitizes query parameters
func SanitizeQueryParam(param string) string {
	// Remove potentially dangerous characters
	param = strings.TrimSpace(param)
	param = strings.ReplaceAll(param, "\n", "")
	param = strings.ReplaceAll(param, "\r", "")
	param = strings.ReplaceAll(param, "\t", "")
	param = strings.ReplaceAll(param, "\x00", "")

	// Limit length
	if len(param) > 100 {
		param = param[:100]
	}

	return param
}

// ValidateProtobufActionType validates action type from protobuf messages
func ValidateProtobufActionType(actionType uint32) error {
	// Define valid action types (adjust based on your protocol)
	validTypes := map[uint32]bool{
		1: true, // ACTION_MINT
		2: true, // ACTION_INVOICE
		3: true, // ACTION_PAYMENT
		// Add other valid action types as needed
	}

	if !validTypes[actionType] {
		return fmt.Errorf("invalid action type: %d", actionType)
	}

	return nil
}

// ValidateProtobufQuantity validates quantity from protobuf messages
func ValidateProtobufQuantity(quantity int32) error {
	if quantity == 0 {
		return fmt.Errorf("quantity must be greater than 0")
	}

	if quantity > MaxQuantity {
		return fmt.Errorf("quantity exceeds maximum value of %d", MaxQuantity)
	}

	return nil
}

// ValidateProtobufHash validates hash from protobuf messages
func ValidateProtobufHash(hash string) error {
	return ValidateHash(hash)
}

// ValidateProtobufAddress validates address from protobuf messages
func ValidateProtobufAddress(address string) error {
	return ValidateAddress(address)
}
