package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var mongoClient *mongo.Client

var rootCmd = &cobra.Command{
	Use:   "amazing-brain-dead-storage-accessor",
	PersistentPreRunE: runRootPersistentPreRunE,
	PersistentPostRunE: runRootPersistentPostRunE,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	rootCmd.AddCommand(
		loginCmd,
		sharedboxCmd,
		)
}

func runRootPersistentPreRunE(cmd *cobra.Command, args []string) error {
	cmdCtx := cmd.Context()
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		return fmt.Errorf("MONGO_URI environment variable is not set")
	}
	connectCtx, connectCancel := context.WithTimeout(cmdCtx, 10*time.Second)
	defer connectCancel()
	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(uri))
	if err != nil {
		return fmt.Errorf("Failed to connect to MongoDB: %v", err)
	}
	pingCtx, pingCancel := context.WithTimeout(cmdCtx, 5*time.Second)
	defer pingCancel()
	if err = client.Ping(pingCtx, nil); err != nil {
		return fmt.Errorf("Failed to ping MongoDB: %w", err)
	}

	mongoClient = client
	log.Println("Successfully connected and pinged MongoDB")
	return nil
}

func runRootPersistentPostRunE(cmd *cobra.Command, args []string) error {
	if mongoClient != nil {
		cmdCtx := cmd.Context()
		disconnectCtx, disconnectCancel := context.WithTimeout(cmdCtx, 10*time.Second)
		defer disconnectCancel()
		if err := mongoClient.Disconnect(disconnectCtx); err != nil {
			log.Printf("Failed to disconnect from MongoDB: %v", err)
		}
	}
	return nil
}

