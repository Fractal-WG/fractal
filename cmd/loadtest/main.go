package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/store"
)

type LoadTestConfig struct {
	MintsCount            int
	UnconfirmedMintsCount int
	OffersPerMint         int
	InvoicesPerMint       int
	BalancesPerMint       int
	Addresses             int
	ConcurrentWorkers     int
	QueryIterations       int
	QueryConcurrency      int
	DatabaseURL           string
}

type QueryResult struct {
	QueryName    string
	Duration     time.Duration
	RowsReturned int
	Error        error
}

type TestReport struct {
	TotalDuration  time.Duration
	DataGeneration time.Duration
	QueryTests     []QueryResult
	SlowQueries    []QueryResult
	DatabaseURL    string
	RecordCounts   map[string]int
}

var (
	numMints         = flag.Int("mints", 1000, "Number of mints to create")
	unconfirmedMints = flag.Int("unconfirmed-mints", 100, "Number of unconfirmed mints to create")
	offersPerMint    = flag.Int("offers", 10, "Number of buy/sell offers per mint")
	invoicesPerMint  = flag.Int("invoices", 5, "Number of invoices per mint")
	balancesPerMint  = flag.Int("balances", 20, "Number of token balances per mint")
	numAddresses     = flag.Int("addresses", 500, "Number of unique addresses")
	workers          = flag.Int("workers", 4, "Number of concurrent workers for data generation")
	iterations       = flag.Int("iterations", 100, "Number of iterations for each query test")
	queryConcurrency = flag.Int("query-concurrency", 1, "Number of concurrent workers per query test")
	dbURL            = flag.String("db", "", "Database URL (defaults to DATABASE_URL env or config default)")
	cleanDB          = flag.Bool("clean", false, "Clean database before running test")
	skipData         = flag.Bool("skip-data", false, "Skip data generation (use existing data)")
	reportFile       = flag.String("report", "loadtest-report.json", "Output file for test report")
)

func main() {
	flag.Parse()

	cfg := LoadTestConfig{
		MintsCount:            *numMints,
		UnconfirmedMintsCount: *unconfirmedMints,
		OffersPerMint:         *offersPerMint,
		InvoicesPerMint:       *invoicesPerMint,
		BalancesPerMint:       *balancesPerMint,
		Addresses:             *numAddresses,
		ConcurrentWorkers:     *workers,
		QueryIterations:       *iterations,
		QueryConcurrency:      *queryConcurrency,
		DatabaseURL:           *dbURL,
	}

	if cfg.DatabaseURL == "" {
		if envURL := os.Getenv("DATABASE_URL"); envURL != "" {
			cfg.DatabaseURL = envURL
		}
	}

	log.Printf("Starting load test with configuration:")
	log.Printf("  Database URL: %s", cfg.DatabaseURL)
	log.Printf("  Mints: %d", cfg.MintsCount)
	log.Printf("  Unconfirmed mints: %d", cfg.UnconfirmedMintsCount)
	log.Printf("  Offers per mint: %d", cfg.OffersPerMint)
	log.Printf("  Invoices per mint: %d", cfg.InvoicesPerMint)
	log.Printf("  Balances per mint: %d", cfg.BalancesPerMint)
	log.Printf("  Unique addresses: %d", cfg.Addresses)
	log.Printf("  Concurrent workers: %d", cfg.ConcurrentWorkers)
	log.Printf("  Query iterations: %d", cfg.QueryIterations)
	log.Printf("  Query concurrency: %d", cfg.QueryConcurrency)

	startTime := time.Now()

	// Connect to database
	appConfig := config.NewConfig()
	appConfig.DatabaseURL = cfg.DatabaseURL
	db, err := store.NewTokenisationStore(cfg.DatabaseURL, *appConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations
	err = db.Migrate()
	if err != nil && err.Error() != "no change" {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean database if requested
	if *cleanDB {
		log.Println("Cleaning database...")
		if err := cleanDatabase(db); err != nil {
			log.Fatalf("Failed to clean database: %v", err)
		}
	}

	report := &TestReport{
		DatabaseURL:  cfg.DatabaseURL,
		RecordCounts: make(map[string]int),
	}

	// Generate test data
	if !*skipData {
		log.Println("Generating test data...")
		dataStartTime := time.Now()
		if err := generateTestData(db, cfg); err != nil {
			log.Fatalf("Failed to generate test data: %v", err)
		}
		report.DataGeneration = time.Since(dataStartTime)
		log.Printf("Data generation completed in %v", report.DataGeneration)
	} else {
		log.Println("Skipping data generation (using existing data)")
	}

	// Count records
	report.RecordCounts = countRecords(db)
	log.Printf("Record counts: %+v", report.RecordCounts)

	// Run query performance tests
	log.Println("\nRunning query performance tests...")
	report.QueryTests = runQueryTests(db, cfg)

	// Identify slow queries (threshold: 100ms)
	for _, result := range report.QueryTests {
		if result.Duration > 100*time.Millisecond {
			report.SlowQueries = append(report.SlowQueries, result)
		}
	}

	report.TotalDuration = time.Since(startTime)

	// Print results
	printReport(report)

	// Save report to file
	if err := saveReport(report, *reportFile); err != nil {
		log.Printf("Warning: Failed to save report to file: %v", err)
	} else {
		log.Printf("\nReport saved to %s", *reportFile)
	}
}

func cleanDatabase(db *store.TokenisationStore) error {
	tables := []string{
		"invoice_signatures",
		"pending_token_balances",
		"token_balances",
		"unconfirmed_invoices",
		"invoices",
		"buy_offers",
		"sell_offers",
		"unconfirmed_mints",
		"mints",
		"onchain_transactions",
		"chain_position",
		"health",
	}

	for _, table := range tables {
		if _, err := db.DB.Exec(fmt.Sprintf("DELETE FROM %s", table)); err != nil {
			return fmt.Errorf("failed to clean table %s: %w", table, err)
		}
		log.Printf("  Cleaned table: %s", table)
	}

	return nil
}

func generateTestData(db *store.TokenisationStore, cfg LoadTestConfig) error {
	// Generate addresses
	addresses := make([]string, cfg.Addresses)
	for i := 0; i < cfg.Addresses; i++ {
		addresses[i] = generateAddress()
	}

	// Generate mints with concurrent workers
	var wg sync.WaitGroup
	mintChan := make(chan int, cfg.MintsCount)
	hashChan := make(chan struct{}, cfg.MintsCount)

	// Start workers
	for w := 0; w < cfg.ConcurrentWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range mintChan {
				_, err := createMintBundle(db, i, addresses, cfg)
				if err != nil {
					log.Printf("Error creating mint bundle %d: %v", i, err)
					continue
				}
				hashChan <- struct{}{}
			}
		}()
	}

	// Send work to workers
	go func() {
		for i := 0; i < cfg.MintsCount; i++ {
			mintChan <- i
		}
		close(mintChan)
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(hashChan)
	}()

	created := 0
	for range hashChan {
		created++
		if created%100 == 0 {
			log.Printf("  Created %d/%d mints", created, cfg.MintsCount)
		}
	}

	log.Printf("Created %d mints with offers, invoices, and balances", created)

	if cfg.UnconfirmedMintsCount > 0 {
		log.Println("Creating unconfirmed mints...")
		for i := 0; i < cfg.UnconfirmedMintsCount; i++ {
			if _, err := createUnconfirmedMint(db, i, addresses); err != nil {
				log.Printf("Error creating unconfirmed mint %d: %v", i, err)
			}
			if (i+1)%100 == 0 {
				log.Printf("  Created %d/%d unconfirmed mints", i+1, cfg.UnconfirmedMintsCount)
			}
		}
		log.Printf("Created %d unconfirmed mints", cfg.UnconfirmedMintsCount)
	}

	return nil
}

func createMintBundle(db *store.TokenisationStore, index int, addresses []string, cfg LoadTestConfig) (string, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return "", err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	hash, err := createMintWithTx(db, index, addresses, tx)
	if err != nil {
		return "", err
	}

	for j := 0; j < cfg.OffersPerMint; j++ {
		if err := createSellOfferWithTx(db, hash, addresses, tx); err != nil {
			return "", err
		}
		if err := createBuyOfferWithTx(db, hash, addresses, tx); err != nil {
			return "", err
		}
	}

	for j := 0; j < cfg.InvoicesPerMint; j++ {
		if err := createInvoiceWithTx(db, hash, addresses, tx); err != nil {
			return "", err
		}
	}

	for j := 0; j < cfg.BalancesPerMint; j++ {
		address := addresses[rand.Intn(len(addresses))]
		balance := int(rand.Int63n(10000) + 1)
		if err := db.UpsertTokenBalanceWithTransaction(address, hash, balance, tx); err != nil {
			return "", err
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	committed = true

	return hash, nil
}

func createMintWithTx(db *store.TokenisationStore, index int, addresses []string, tx *sql.Tx) (string, error) {
	hash := generateHash()
	mint := &store.MintWithoutID{
		Hash:                     hash,
		Title:                    fmt.Sprintf("Load Test Mint %d", index),
		Description:              fmt.Sprintf("This is a load test mint created at %s", time.Now().Format(time.RFC3339)),
		FractionCount:            int(rand.Int63n(1000000) + 1),
		Tags:                     store.StringArray{"loadtest", "test"},
		Metadata:                 store.StringInterfaceMap{"test": true, "index": index},
		TransactionHash:          generateHash(),
		Requirements:             store.StringInterfaceMap{},
		LockupOptions:            store.StringInterfaceMap{},
		FeedURL:                  fmt.Sprintf("https://example.com/feed/%d", index),
		OwnerAddress:             addresses[rand.Intn(len(addresses))],
		PublicKey:                generateHash(),
		ContractOfSale:           "Standard contract",
		SignatureRequirementType: store.SignatureRequirementType_ONE_SIGNATURE,
		AssetManagers: store.AssetManagers{
			{
				Name:      "Test Manager",
				PublicKey: generateHash(),
				URL:       "https://example.com/manager",
			},
		},
		MinSignatures: 1,
	}

	_, err := db.SaveMintWithTx(mint, mint.OwnerAddress, tx)
	return hash, err
}

func createUnconfirmedMint(db *store.TokenisationStore, index int, addresses []string) (string, error) {
	hash := generateHash()
	mint := &store.MintWithoutID{
		Hash:                     hash,
		Title:                    fmt.Sprintf("Load Test Unconfirmed Mint %d", index),
		Description:              fmt.Sprintf("This is a load test unconfirmed mint created at %s", time.Now().Format(time.RFC3339)),
		FractionCount:            int(rand.Int63n(1000000) + 1),
		Tags:                     store.StringArray{"loadtest", "test", "unconfirmed"},
		Metadata:                 store.StringInterfaceMap{"test": true, "index": index},
		TransactionHash:          generateHash(),
		Requirements:             store.StringInterfaceMap{},
		LockupOptions:            store.StringInterfaceMap{},
		FeedURL:                  fmt.Sprintf("https://example.com/unconfirmed/feed/%d", index),
		OwnerAddress:             addresses[rand.Intn(len(addresses))],
		PublicKey:                generateHash(),
		ContractOfSale:           "Standard contract",
		SignatureRequirementType: store.SignatureRequirementType_ONE_SIGNATURE,
		AssetManagers: store.AssetManagers{
			{
				Name:      "Test Manager",
				PublicKey: generateHash(),
				URL:       "https://example.com/manager",
			},
		},
		MinSignatures: 1,
	}

	_, err := db.SaveUnconfirmedMint(mint)
	return hash, err
}

func createSellOfferWithTx(db *store.TokenisationStore, mintHash string, addresses []string, tx *sql.Tx) error {
	offer := &store.SellOfferWithoutID{
		OffererAddress: addresses[rand.Intn(len(addresses))],
		MintHash:       mintHash,
		Quantity:       int(rand.Int63n(1000) + 1),
		Price:          int(rand.Int63n(1000000) + 1),
		PublicKey:      generateHash(),
	}
	_, err := db.SaveSellOfferWithTx(offer, tx)
	return err
}

func createBuyOfferWithTx(db *store.TokenisationStore, mintHash string, addresses []string, tx *sql.Tx) error {
	offer := &store.BuyOfferWithoutID{
		OffererAddress: addresses[rand.Intn(len(addresses))],
		SellerAddress:  addresses[rand.Intn(len(addresses))],
		MintHash:       mintHash,
		Quantity:       int(rand.Int63n(1000) + 1),
		Price:          int(rand.Int63n(1000000) + 1),
		PublicKey:      generateHash(),
	}
	_, err := db.SaveBuyOfferWithTx(offer, tx)
	return err
}

func createInvoiceWithTx(db *store.TokenisationStore, mintHash string, addresses []string, tx *sql.Tx) error {
	invoice := &store.Invoice{
		Hash:            generateHash(),
		PaymentAddress:  addresses[rand.Intn(len(addresses))],
		BuyerAddress:    addresses[rand.Intn(len(addresses))],
		SellerAddress:   addresses[rand.Intn(len(addresses))],
		MintHash:        mintHash,
		Quantity:        int(rand.Int63n(100) + 1),
		Price:           int(rand.Int63n(100000) + 1),
		CreatedAt:       time.Now(),
		BlockHeight:     rand.Int63n(1000000),
		TransactionHash: generateHash(),
		PublicKey:       generateHash(),
		Signature:       generateHash(),
	}
	_, err := db.SaveInvoiceWithTx(invoice, tx)
	return err
}

func countRecords(db *store.TokenisationStore) map[string]int {
	counts := make(map[string]int)
	tables := []string{
		"mints",
		"sell_offers",
		"buy_offers",
		"invoices",
		"token_balances",
		"unconfirmed_mints",
		"unconfirmed_invoices",
		"onchain_transactions",
	}

	for _, table := range tables {
		var count int
		err := db.DB.QueryRowContext(context.Background(), fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err != nil {
			log.Printf("Warning: Failed to count %s: %v", table, err)
			continue
		}
		counts[table] = count
	}

	return counts
}

func runQueryTests(db *store.TokenisationStore, cfg LoadTestConfig) []QueryResult {
	var results []QueryResult

	// Test 1: GetMints (paginated)
	results = append(results, testQuery("GetMints(limit=50)", func() (int, error) {
		mints, err := db.GetMints(50, 0)
		return len(mints), err
	}, cfg.QueryIterations, cfg.QueryConcurrency))

	// Test 2: GetMints (large page)
	results = append(results, testQuery("GetMints(limit=500)", func() (int, error) {
		mints, err := db.GetMints(500, 0)
		return len(mints), err
	}, cfg.QueryIterations, cfg.QueryConcurrency))

	// Test 3: GetMintByHash
	hash := getRandomMintHash(db)
	if hash != "" {
		results = append(results, testQuery("GetMintByHash", func() (int, error) {
			mint, err := db.GetMintByHash(hash)
			if err != nil {
				return 0, err
			}
			if mint.Hash == "" {
				return 0, nil
			}
			return 1, nil
		}, cfg.QueryIterations, cfg.QueryConcurrency))
	}

	// Test 4: GetMintsByPublicKey (confirmed only)
	mintPublicKey := getRandomMintPublicKey(db)
	if mintPublicKey != "" {
		results = append(results, testQuery("GetMintsByPublicKey(confirmed)", func() (int, error) {
			mints, err := db.GetMintsByPublicKey(0, 100, mintPublicKey, false)
			return len(mints), err
		}, cfg.QueryIterations, cfg.QueryConcurrency))
	}

	// Test 5: GetMintsByPublicKey (include unconfirmed)
	unconfirmedPublicKey := getRandomUnconfirmedPublicKey(db)
	if unconfirmedPublicKey != "" {
		results = append(results, testQuery("GetMintsByPublicKey(includeUnconfirmed)", func() (int, error) {
			mints, err := db.GetMintsByPublicKey(0, 100, unconfirmedPublicKey, true)
			return len(mints), err
		}, cfg.QueryIterations, cfg.QueryConcurrency))
	}

	// Test 6: GetMintsByAddress (confirmed only)
	address := getRandomAddress(db)
	if address != "" {
		results = append(results, testQuery("GetMintsByAddress(confirmed)", func() (int, error) {
			mints, err := db.GetMintsByAddress(0, 100, address, false)
			return len(mints), err
		}, cfg.QueryIterations, cfg.QueryConcurrency))
	}

	// Test 7: GetMintsByAddress (include unconfirmed)
	unconfirmedAddress := getRandomUnconfirmedAddress(db)
	if unconfirmedAddress != "" {
		results = append(results, testQuery("GetMintsByAddress(includeUnconfirmed)", func() (int, error) {
			mints, err := db.GetMintsByAddress(0, 100, unconfirmedAddress, true)
			return len(mints), err
		}, cfg.QueryIterations, cfg.QueryConcurrency))
	}

	// Test 8: GetUnconfirmedMints
	results = append(results, testQuery("GetUnconfirmedMints(limit=100)", func() (int, error) {
		mints, err := db.GetUnconfirmedMints(0, 100)
		return len(mints), err
	}, cfg.QueryIterations, cfg.QueryConcurrency))

	// Test 9: GetSellOffers
	results = append(results, testQuery("GetSellOffers(limit=100)", func() (int, error) {
		offers, err := db.GetSellOffers(0, 100, "", "")
		return len(offers), err
	}, cfg.QueryIterations, cfg.QueryConcurrency))

	// Test 10: GetBuyOffers
	results = append(results, testQuery("GetBuyOffers(limit=100)", func() (int, error) {
		offers, err := db.GetBuyOffersByMintAndSellerAddress(0, 100, "", "")
		return len(offers), err
	}, cfg.QueryIterations, cfg.QueryConcurrency))

	// Test 11: GetInvoices
	results = append(results, testQuery("GetInvoices(limit=100)", func() (int, error) {
		invoices, err := db.GetInvoices(0, 100, "", "")
		return len(invoices), err
	}, cfg.QueryIterations, cfg.QueryConcurrency))

	// Test 12: GetTokenBalances
	if hash != "" {
		address := getRandomAddress(db)
		if address != "" {
			results = append(results, testQuery("GetTokenBalances", func() (int, error) {
				balances, err := db.GetTokenBalances(address, hash)
				if err != nil {
					return 0, err
				}
				return len(balances), nil
			}, cfg.QueryIterations, cfg.QueryConcurrency))
		}
	}

	// Test 13: GetStats
	results = append(results, testQuery("GetStats", func() (int, error) {
		stats, err := db.GetStats()
		if err != nil {
			return 0, err
		}
		if mintsCount, ok := stats["mints"]; ok {
			return mintsCount, nil
		}
		return 0, nil
	}, cfg.QueryIterations, cfg.QueryConcurrency))

	// Test 14: Complex join - GetSellOffers for specific mint
	if hash != "" {
		results = append(results, testQuery("GetSellOffersForMint", func() (int, error) {
			var count int
			err := db.DB.QueryRowContext(context.Background(),
				"SELECT COUNT(*) FROM sell_offers WHERE mint_hash = $1", hash).Scan(&count)
			return count, err
		}, cfg.QueryIterations, cfg.QueryConcurrency))
	}

	// Test 15: Balance aggregation query
	results = append(results, testQuery("AggregateBalancesByMint", func() (int, error) {
		rows, err := db.DB.QueryContext(context.Background(),
			"SELECT mint_hash, SUM(quantity) as total FROM token_balances GROUP BY mint_hash ORDER BY mint_hash LIMIT 100")
		if err != nil {
			return 0, err
		}
		defer rows.Close()

		count := 0
		for rows.Next() {
			var mintHash string
			var total int64
			if err := rows.Scan(&mintHash, &total); err != nil {
				return count, err
			}
			count++
		}
		return count, rows.Err()
	}, cfg.QueryIterations, cfg.QueryConcurrency))

	return results
}

func testQuery(name string, queryFunc func() (int, error), iterations int, concurrency int) QueryResult {
	result := QueryResult{
		QueryName: name,
	}

	if concurrency < 1 {
		concurrency = 1
	}

	type queryRun struct {
		duration time.Duration
		rows     int
		err      error
	}

	results := make(chan queryRun, iterations*concurrency)
	done := make(chan struct{})

	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				select {
				case <-done:
					return
				default:
				}
				start := time.Now()
				rows, err := queryFunc()
				results <- queryRun{
					duration: time.Since(start),
					rows:     rows,
					err:      err,
				}
				if err != nil {
					return
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	totalDuration := time.Duration(0)
	totalRows := 0
	totalRuns := 0

	for run := range results {
		if run.err != nil && result.Error == nil {
			result.Error = run.err
			close(done)
		}
		if run.err != nil {
			continue
		}
		totalDuration += run.duration
		totalRows += run.rows
		totalRuns++
	}

	if totalRuns > 0 {
		result.Duration = totalDuration / time.Duration(totalRuns)
		result.RowsReturned = totalRows / totalRuns
	}

	return result
}

func getRandomMintHash(db *store.TokenisationStore) string {
	var hash string
	err := db.DB.QueryRowContext(context.Background(),
		"SELECT hash FROM mints ORDER BY RANDOM() LIMIT 1").Scan(&hash)
	if err != nil {
		return ""
	}
	return hash
}

func getRandomMintPublicKey(db *store.TokenisationStore) string {
	var publicKey string
	err := db.DB.QueryRowContext(context.Background(),
		"SELECT public_key FROM mints ORDER BY RANDOM() LIMIT 1").Scan(&publicKey)
	if err != nil {
		return ""
	}
	return publicKey
}

func getRandomAddress(db *store.TokenisationStore) string {
	var address string
	err := db.DB.QueryRowContext(context.Background(),
		"SELECT owner_address FROM mints ORDER BY RANDOM() LIMIT 1").Scan(&address)
	if err != nil {
		return ""
	}
	return address
}

func getRandomUnconfirmedPublicKey(db *store.TokenisationStore) string {
	var publicKey string
	err := db.DB.QueryRowContext(context.Background(),
		"SELECT public_key FROM unconfirmed_mints ORDER BY RANDOM() LIMIT 1").Scan(&publicKey)
	if err != nil {
		return ""
	}
	return publicKey
}

func getRandomUnconfirmedAddress(db *store.TokenisationStore) string {
	var address string
	err := db.DB.QueryRowContext(context.Background(),
		"SELECT owner_address FROM unconfirmed_mints ORDER BY RANDOM() LIMIT 1").Scan(&address)
	if err != nil {
		return ""
	}
	return address
}

func printReport(report *TestReport) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("LOAD TEST REPORT")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Database: %s\n", report.DatabaseURL)
	fmt.Printf("Total Duration: %v\n", report.TotalDuration)
	fmt.Printf("Data Generation: %v\n", report.DataGeneration)
	fmt.Println()

	fmt.Println("Record Counts:")
	for table, count := range report.RecordCounts {
		fmt.Printf("  %-30s %10d\n", table+":", count)
	}
	fmt.Println()

	fmt.Println("Query Performance Results:")
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("%-40s %15s %15s\n", "Query", "Avg Duration", "Avg Rows")
	fmt.Println(strings.Repeat("-", 80))

	for _, result := range report.QueryTests {
		status := ""
		if result.Error != nil {
			status = fmt.Sprintf(" [ERROR: %v]", result.Error)
		} else if result.Duration > 100*time.Millisecond {
			status = " [SLOW]"
		}
		fmt.Printf("%-40s %15v %15d%s\n",
			result.QueryName,
			result.Duration,
			result.RowsReturned,
			status)
	}
	fmt.Println()

	if len(report.SlowQueries) > 0 {
		fmt.Println("SLOW QUERIES (>100ms):")
		fmt.Println(strings.Repeat("-", 80))
		for _, result := range report.SlowQueries {
			fmt.Printf("  %-40s %15v\n", result.QueryName, result.Duration)
		}
		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 80))
}

func saveReport(report *TestReport, filename string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func generateHash() string {
	const chars = "0123456789abcdef"
	hash := make([]byte, 64)
	for i := range hash {
		hash[i] = chars[rand.Intn(len(chars))]
	}
	return string(hash)
}

func generateAddress() string {
	// Generate a Dogecoin-like address
	const chars = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	addr := make([]byte, 34)
	addr[0] = 'D' // Dogecoin addresses start with D
	for i := 1; i < 34; i++ {
		addr[i] = chars[rand.Intn(len(chars))]
	}
	return string(addr)
}
