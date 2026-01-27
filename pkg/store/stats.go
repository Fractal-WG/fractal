package store

import (
	"context"
	"fmt"
)

func (s *TokenisationStore) GetStats(ctx context.Context) (map[string]int, error) {
	stats := make(map[string]int)

	mints, err := getMintsCount(ctx, s)
	if err != nil {
		fmt.Println("Error getting mints count:", err)
		return nil, err
	}

	unconfirmedMints, err := getUnconfirmedMintsCount(ctx, s)
	if err != nil {
		fmt.Println("Error getting unconfirmed mints count:", err)
		return nil, err
	}

	onChainTransactions, err := getOnChainTransactionsCount(ctx, s)
	if err != nil {
		fmt.Println("Error getting onchain transactions count:", err)
		return nil, err
	}

	stats["mints"] = mints
	stats["unconfirmed_mints"] = unconfirmedMints
	stats["onchain_transactions"] = onChainTransactions

	return stats, nil

}
