package cmd

import "context"

func fetchSharedBoxListRecursively(ctx context.Context, initialNode string) ([]SharedBoxListItem, error) {
	logger.InfoContext(ctx, "Fetching sharedbox list recursively", "initialNode", initialNode)
	var allItems []SharedBoxListItem
	return allItems, nil
}
