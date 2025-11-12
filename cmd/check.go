package cmd

import (
	"context"
	"fmt"
	"log"
	"time"
	"os"

	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (target string)

func init() {
	checkCmd.Flags().StringVarP(&target, "target", "t", "", "Target to check")
	err := checkCmd.MarkFlagRequired("target")
	if err != nil {
		log.Fatalf("Failed to mark target flag as required: %v", err)
	}
}

var checkCmd = &cobra.Command{
	Use:   "check",
	RunE: runCheckCmd,
}

func checkMongoDB(ctx context.Context) error {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		return fmt.Errorf("MONGO_URI environment variable is not set")
	}

	connectCtx, connectCancel := context.WithTimeout(ctx, 10*time.Second)
	defer connectCancel()
	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(uri))
	if err != nil {
		return fmt.Errorf("Failed to connect to MongoDB: %v", err)
	}

	disconnectCtx, disconnectCancel := context.WithTimeout(ctx, 10*time.Second)
	defer disconnectCancel()
	defer func() {
		if err = client.Disconnect(disconnectCtx); err != nil {
			log.Printf("Failed to disconnect from MongoDB: %v", err)
		}
	}()

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err = client.Ping(pingCtx, nil); err != nil {
		return fmt.Errorf("Failed to ping MongoDB: %v", err)
	}
	fmt.Println("Successfully connected and pinged MongoDB!")
	return nil
}

func runCheckCmd(cmd *cobra.Command, args []string) error {
	cmdCtx := cmd.Context()
	if target == "mongodb" {
		return checkMongoDB(cmdCtx)
	}
	return fmt.Errorf("Invalid target: %s", target)
}

