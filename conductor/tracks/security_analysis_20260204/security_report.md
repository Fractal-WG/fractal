# Preliminary Security Scan Report for Fractal Engine

This report summarizes the findings from the automated security scans performed using `govulncheck` and `gosec` on the Fractal Engine codebase.

---

## 1. `govulncheck` Findings (Directly Affecting Code)

`govulncheck` identifies known vulnerabilities in Go modules and their dependencies, focusing on instances where the vulnerable functions are actually called by the codebase.

### Critical Severity:

*   **GO-2024-3189: Consensus failure in `github.com/btcsuite/btcd`**
    *   **Impact:** Could lead to blockchain forks or incorrect transaction validation, compromising the integrity of the system.
    *   **Affected versions:** `github.com/btcsuite/btcd@v0.20.1-beta`
    *   **Fixed in:** `github.com/btcsuite/btcd@v0.24.2-beta.rc1`
    *   **Recommendation:** Upgrade `github.com/btcsuite/btcd` to `v0.24.2-beta.rc1` or higher.
    *   **Location:** Traced to `pkg/doge/doge.go`.

*   **GO-2024-2818: Consensus failures in `github.com/btcsuite/btcd`**
    *   **Impact:** Similar to GO-2024-3189, potential for blockchain instability and compromised transaction processing.
    *   **Affected versions:** `github.com/btcsuite/btcd@v0.20.1-beta`
    *   **Fixed in:** `github.com/btcsuite/btcd@v0.24.0`
    *   **Recommendation:** Upgrade `github.com/btcsuite/btcd` to `v0.24.2-beta.rc1` or higher (this version includes the fix).
    *   **Location:** Traced to `pkg/doge/doge.go`.

### High Severity:

*   **GO-2026-4341: Memory exhaustion in `net/url` (Standard library)**
    *   **Impact:** Potential for denial of service (DoS) if specially crafted URLs are processed, leading to resource exhaustion.
    *   **Affected versions:** `net/url@go1.25.5`
    *   **Fixed in:** `net/url@go1.25.6`
    *   **Recommendation:** Upgrade Go version to `go1.25.6` or higher.
    *   **Location:** Traced to `internal/test/support/shared_db.go` and `pkg/indexer/indexer.go`.

*   **GO-2026-4340: Handshake messages may be processed at the incorrect encryption level in `crypto/tls` (Standard library)**
    *   **Impact:** Potential security bypass or data exposure during TLS handshake, leading to weakened encryption or authentication.
    *   **Affected versions:** `crypto/tls@go1.25.5`
    *   **Fixed in:** `crypto/tls@go1.25.6`
    *   **Recommendation:** Upgrade Go version to `go1.25.6` or higher.
    *   **Location:** Traced to `pkg/store/invoices.go`, `pkg/rpc/server.go`, `pkg/dogenet/client.go`, `pkg/store/debug.go`, `internal/test/support/shared_db.go`, `pkg/doge/rpc.go`.

*   **GO-2022-1098: Denial of service in message decoding in `github.com/btcsuite/btcd`**
    *   **Impact:** Potential for network disruption or crash if malformed messages are processed.
    *   **Affected versions:** `github.com/btcsuite/btcd@v0.20.1-beta`
    *   **Fixed in:** `github.com/btcsuite/btcd@v0.23.2`
    *   **Recommendation:** Upgrade `github.com/btcsuite/btcd` to `v0.24.2-beta.rc1` or higher (this version includes the fix).
    *   **Location:** Traced to `pkg/doge/doge.go`.

---

## 2. `gosec` Findings

`gosec` performs static analysis of the Go source code to identify common programming mistakes and security vulnerabilities.

### High Severity:

*   **G115 (CWE-190): Integer Overflow (int64 -> int32 or int -> int32 conversion)**
    *   **Impact:** Data corruption or unexpected behavior if the value exceeds the maximum capacity of an `int32`. This can lead to incorrect calculations or state.
    *   **Recommendation:** Review all identified conversion points. If the source value can exceed the `int32` maximum, ensure that a larger type (e.g., `int64`) is used or implement explicit range checks to handle potential overflows gracefully.
    *   **Location:** Numerous instances found in `pkg/rpc/health.go`, `pkg/rpc/connect_conversions.go`, `pkg/rpc/stats.go`, `pkg/rpc/offers.go`, `pkg/rpc/mints.go`, `pkg/rpc/invoices.go`, `pkg/dogenet/sell_offers.go`, `pkg/dogenet/mint.go`, `pkg/dogenet/invoices.go`, `pkg/dogenet/buy_offers.go`, `pkg/cli/commands/invoices.go`, `internal/test/stack/support.go`.

*   **G404 (CWE-338): Use of weak random number generator (`math/rand`)**
    *   **Impact:** Predictable randomness can undermine cryptographic operations, session token generation, or other security-sensitive functions.
    *   **Recommendation:** Replace `math/rand` with `crypto/rand` for any random number generation used in security-sensitive contexts where unpredictability is crucial.
    *   **Location:** `internal/test/support/support.go`, `cmd/loadtest/main.go`.

*   **G101 (CWE-798): Potential hardcoded credentials**
    *   **Impact:** While these appear to be RPC procedure names (which are less likely to be sensitive), hardcoded strings could potentially expose sensitive information or system architecture details.
    *   **Recommendation:** Verify that the identified strings are indeed not sensitive. If any are, they should be moved to secure configuration management.
    *   **Location:** `pkg/rpc/protocol/protocolconnect/rpc.connect.go`.

*   **G109 (CWE-190): Potential Integer overflow made by `strconv.Atoi` result conversion to `int16/32`**
    *   **Impact:** Similar to G115, incorrect handling of string-to-integer conversions can lead to overflows if the string represents a value outside the target integer type's range.
    *   **Recommendation:** Validate the input string from `strconv.Atoi` to ensure it fits within the target `int32` range before conversion, or use a larger integer type if appropriate.
    *   **Location:** `pkg/cli/commands/invoices.go`.

### Medium Severity:

*   **G406 (CWE-328) & G507 (CWE-327): Use of deprecated weak cryptographic primitive (RIPEMD160)**
    *   **Impact:** RIPEMD160 is considered cryptographically weak and may be vulnerable to collision attacks, compromising data integrity or authenticity.
    *   **Recommendation:** Replace `golang.org/x/crypto/ripemd160` with a stronger, modern cryptographic hash function such as SHA256 (from `crypto/sha256`).
    *   **Location:** `pkg/doge/doge.go`.

*   **G202 (CWE-89): SQL string concatenation**
    *   **Impact:** High risk of SQL injection if user-controlled input is directly concatenated into SQL queries, allowing attackers to manipulate database queries.
    *   **Recommendation:** Always use parameterized queries or an Object-Relational Mapper (ORM) to prevent SQL injection vulnerabilities. Avoid direct string concatenation for dynamic query parts.
    *   **Location:** `pkg/store/debug.go`.

*   **G304 (CWE-22): Potential file inclusion via variable**
    *   **Impact:** Allows an attacker to read or execute arbitrary files on the server through directory traversal techniques.
    *   **Recommendation:** Sanitize file paths using `filepath.Clean` and ensure that file access is restricted to a predefined, safe directory. Consider using `os.DirFS` or similar mechanisms for Go 1.16+ to create a virtual file system root.
    *   **Location:** `internal/test/stack/support.go`.

*   **G112 (CWE-400): Potential Slowloris Attack (`ReadHeaderTimeout` not configured)**
    *   **Impact:** The HTTP server may be vulnerable to Slowloris-like denial-of-service attacks, where clients hold connections open by sending partial requests slowly.
    *   **Recommendation:** Configure `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, and `IdleTimeout` on the `http.Server` to set limits on request processing times and prevent resource exhaustion.
    *   **Location:** `pkg/rpc/server.go`.

*   **G306 (CWE-276): Expect `WriteFile` permissions to be `0600` or less**
    *   **Impact:** Overly permissive file permissions (`0644`) can allow unauthorized users or processes to read or modify sensitive files.
    *   **Recommendation:** Use more restrictive file permissions (e.g., `0600` for owner-only access or `0640` for owner/group read) when writing files, especially those containing sensitive data.
    *   **Location:** `pkg/cli/config.go`, `cmd/loadtest/main.go`.

### Low Severity:

*   **G103 (CWE-242): Use of unsafe calls should be audited**
    *   **Impact:** Direct use of `unsafe` operations can bypass Go's memory safety guarantees, potentially leading to crashes, memory corruption, or undefined behavior if not used with extreme care. These are commonly found in generated code (e.g., protobufs).
    *   **Recommendation:** Verify that these generated protobuf files are from trusted sources and that their use of `unsafe` is intentional and thoroughly reviewed by the protobuf tool maintainers.
    *   **Location:** `pkg/rpc/protocol/*.pb.go`, `pkg/protocol/*.pb.go`.

*   **G104 (CWE-703): Errors unhandled**
    *   **Impact:** Unhandled errors can lead to unexpected program behavior, resource leaks, or obscured root causes of failures. This can also mask security-critical failures.
    *   **Recommendation:** Ensure all returned errors are explicitly handled. This can involve checking the error and logging it, returning it to the caller, or taking corrective action. If an error is intentionally ignored, add a comment explaining the rationale.
    *   **Location:** Many files across `pkg/store`, `pkg/service`, `pkg/rpc`, `pkg/protocol`, `pkg/dogenet`, `pkg/cli/commands`, `internal/test/support`, `cmd/cli`.
