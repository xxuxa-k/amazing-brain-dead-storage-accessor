package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var sharedboxImportCmd = &cobra.Command{
	Use:   "import",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if err := initAdminApiClient(); err != nil {
			return fmt.Errorf("Failed to initialize Admin API client: %v", err)
		}
		logger.InfoContext(cmd.Context(), "Initialized Admin API client")
		return nil
	},
	RunE: runSharedboxImportCmd,
}

func init() {
	sharedboxImportCmd.Flags().Bool("recursive", false, "Import sharedboxes recursively")
}

func newListApiUrl(node string) (string, error) {
	baseURL := "https://api.directcloud.jp/openapp/m1/sharedboxes/lists"
	if node == "" {
		return baseURL, nil
	}
	u, err := url.Parse(fmt.Sprintf("https://api.directcloud.jp/openapp/m1/sharedboxes/lists/%s", node))
	if err != nil {
		return "", fmt.Errorf("Failed to parse URL: %w", err)
	}
	params := url.Values{}
	params.Add("lang", "eng")
	u.RawQuery = params.Encode()
	return u.String(), nil
}

func fetchSharedBoxList(cmdCtx context.Context, node string) (*SharedBoxListResponse, error) {
	result := SharedBoxListResponse{}
	u, err := newListApiUrl(node)
	if err != nil {
		return &result, fmt.Errorf("Failed to construct API URL: %w", err)
	}
	req, err := adminApiClient.NewGetRequest(u)
	if err != nil {
		return &result, fmt.Errorf("Failed to create GET request: %w", err)
	}
	req = req.WithContext(cmdCtx)
	resp, err := adminApiClient.httpClient.Do(req)
	if err != nil {
		return &result, fmt.Errorf("Failed to send GET request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &result, fmt.Errorf("Failed to read response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return &result, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return &result, fmt.Errorf("Failed to parse response JSON: %w", err)
	}
	return &result, nil
}

func runSharedboxImportCmd(cmd *cobra.Command, args []string) error {
	node, err := cmd.Flags().GetString("node")
	if err != nil {
		return fmt.Errorf("Failed to get 'node' flag: %v", err)
	}
	recursive, _ := cmd.Flags().GetBool("recursive")
	logger.Info("sharedbox import", "node", node, "recursive", recursive)

	var allItems []SharedBoxListItem

	if recursive {
		allItems, err = fetchSharedBoxListRecursively(cmd.Context(), node)
		if err != nil {
			return fmt.Errorf("Failed to fetch sharedbox list recursively: %w", err)
		}
	} else {
		result, err := fetchSharedBoxList(cmd.Context(), node)
		if err != nil {
			return fmt.Errorf("Failed to fetch sharedbox list: %w", err)
		}
		if !result.Success {
			return fmt.Errorf("API request unsuccessful")
		}
		allItems = result.Lists
	}

	writeModels := make([]mongo.WriteModel, len(allItems))
	for i, item := range allItems {
		model := mongo.NewReplaceOneModel().SetFilter(bson.M{
			"node": item.Node,
		}).SetReplacement(item).SetUpsert(true)
		writeModels[i] = model
	}

	mongoDatabase := os.Getenv("MONGO_DATABASE")
	if mongoDatabase == "" {
		log.Println("MONGO_DATABASE not set, using default 'sample'")
		mongoDatabase = "sample"
	}
	collection := mongoClient.Database(mongoDatabase).Collection("sharedbox")

	_, err = collection.Indexes().CreateOne(
		cmd.Context(),
		mongo.IndexModel{
			Keys: bson.D{{Key: "node", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		)
	if err != nil {
		logger.Info("Failed to create index on 'node'", "err", err)
	}
	bulkResult, err := collection.BulkWrite(cmd.Context(), writeModels)
	if err != nil {
		return fmt.Errorf("Failed to perform bulk write to MongoDB: %v", err)
	}
	logger.InfoContext(
		cmd.Context(),
		"Bulk write to MongoDB completed",
		"insertedCount", bulkResult.InsertedCount,
		"modifiedCount", bulkResult.ModifiedCount,
		"upsertedCount", bulkResult.UpsertedCount,
		"matchedCount", bulkResult.MatchedCount,
		)
	return nil
}
