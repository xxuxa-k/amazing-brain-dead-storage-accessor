package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var userSyncCmd = &cobra.Command{
	Use:   "sync",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if err := initAdminApiClient(); err != nil {
			return fmt.Errorf("Failed to initialize Admin API client: %v", err)
		}
		slog.DebugContext(cmd.Context(), "Initialized Admin API client")
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmdCtx := cmd.Context()
		resp, err := adminApiClient.UsersList(cmdCtx)
		if err != nil {
			slog.ErrorContext(cmdCtx, "Failed to list users", "error", err)
			return err
		}
		if len(resp.Lists) > 0 {
			slog.DebugContext(cmdCtx, "First user in list", 
				"count", len(resp.Lists),
				)
			collection := mongoClient.Database(mongoDatabase).Collection(MONGO_COLLECTION_USERS)
			var writeModels []mongo.WriteModel
			ctx, cancel := context.WithTimeout(cmdCtx, 30*time.Second)
			defer cancel()
			for _, user := range resp.Lists {
				model := mongo.NewReplaceOneModel().SetFilter(bson.M{
					"user_seq": user.UserSeq,
				}).SetReplacement(user).SetUpsert(true)
				writeModels = append(writeModels, model)
			}
			result, err := collection.BulkWrite(ctx, writeModels)
			if err != nil {
				slog.ErrorContext(ctx, "Bulk write error", "error", err)
				return err
			}
			slog.DebugContext(cmdCtx, "Bulk write result",
				"insertedCount", result.InsertedCount,
				"modifiedCount", result.ModifiedCount,
				"upsertedCount", result.UpsertedCount,
				"matchedCount", result.MatchedCount,
				)
		}
		return nil
	},
}

func init() {
}

