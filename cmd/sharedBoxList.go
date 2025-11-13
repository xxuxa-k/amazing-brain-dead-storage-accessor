package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/xxuxa-k/amazing-brain-dead-storage-accessor/internal"
)

var node string

var sharedboxListCmd = &cobra.Command{
	Use:   "list",
	RunE: runSharedboxListCmd,
}

func init() {
	sharedboxListCmd.Flags().Bool("recursive", false, "List sharedboxes recursively")
	sharedboxListCmd.Flags().StringVar(&node, "node", "", "node ID to list sharedboxes from")
}

func runSharedboxListCmd(cmd *cobra.Command, args []string) error {
	u, err := url.Parse(fmt.Sprintf("https://api.directcloud.jp/openapp/m1/sharedboxes/lists/%s", node))
	if err != nil {
		return fmt.Errorf("Failed to parse URL: %v", err)
	}
	params := url.Values{}
	params.Add("lang", "eng")
	u.RawQuery = params.Encode()

	req, err := internal.NewGetRequest(u.String())
	if err != nil {
		return fmt.Errorf("Failed to create GET request: %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to send GET request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	result := &internal.SharedBoxListResponse{}
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("Failed to parse response JSON: %v", err)
	}

	if !result.Success {
		return fmt.Errorf("API request unsuccessful: %s", string(body))
	}

	var insertDocs []interface{}
	for _, box := range result.Lists {
		insertDocs = append(insertDocs, box)
	}

	collection := mongoClient.Database("sample").Collection("sharedbox")
	insertResult, err := collection.InsertMany(cmd.Context(), insertDocs)
	if err != nil {
		return fmt.Errorf("Failed to insert documents into MongoDB: %v", err)
	}

	fmt.Printf("Inserted %d documents into MongoDB\n", len(insertResult.InsertedIDs))

	return nil
}
