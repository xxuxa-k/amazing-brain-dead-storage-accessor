package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	sessionID string
	logger *slog.Logger
	mongoClient *mongo.Client
	redisClient *redis.Client
	adminApiClient *AdminApiClient
)

var rootCmd = &cobra.Command{
	Use:   "amazing-brain-dead-storage-accessor",
	PersistentPreRunE: func (cmd *cobra.Command, args []string) error {
		cmdCtx := cmd.Context()
		if err := initMongoClient(cmdCtx); err != nil {
			return err
		}
		if err := initRedisClient(cmdCtx); err != nil {
			return err
		}
		return nil
	},
	PersistentPostRunE: func (cmd *cobra.Command, args []string) error {
		var combinedErr error
		cmdCtx := cmd.Context()
		if mongoClient != nil {
			if err := closeMongoClient(cmdCtx); err != nil {
				log.Printf("Error closing MongoDB client: %v", err)
				if combinedErr == nil {
					combinedErr = err
				}
			}
		}
		if redisClient != nil {
			if err := redisClient.Close(); err != nil {
				log.Printf("Error closing Redis client: %v", err)
				if combinedErr == nil {
					combinedErr = err
				}
			}
		}
		return combinedErr
	},
}

func init() {
	sessionID = uuid.New().String()
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("Application started", "sessionID", sessionID)
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}
	rootCmd.PersistentFlags().Bool("verbose", false, "Enable verbose output")
	rootCmd.AddCommand(
		authCmd,
		sharedboxCmd,
		)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func initMongoClient(cmdCtx context.Context) error {
	mongoHost := os.Getenv("MONGO_HOST")
	if mongoHost == "" {
		return fmt.Errorf("MONGO_HOST environment variable is not set")
	}
	mongoPort := os.Getenv("MONGO_PORT")
	if mongoPort == "" {
		return fmt.Errorf("MONGO_PORT environment variable is not set")
	}
	uri := fmt.Sprintf("mongodb://%s:%s", mongoHost, mongoPort)
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
	logger.Info("MongoDB connection established", "host", mongoHost, "port", mongoPort)
	return nil
}
func closeMongoClient(cmdCtx context.Context) error {
	disconnectCtx, disconnectCancel := context.WithTimeout(cmdCtx, 10*time.Second)
	defer disconnectCancel()
	if err := mongoClient.Disconnect(disconnectCtx); err != nil {
		return fmt.Errorf("Failed to disconnect from MongoDB: %v", err)
	}
	return nil
}
func initRedisClient(cmdCtx context.Context) error {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		return fmt.Errorf("REDIS_HOST environment variable is not set")
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		return fmt.Errorf("REDIS_PORT environment variable is not set")
	}
	addr := fmt.Sprintf("%s:%s", redisHost, redisPort)
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	})
	pingCtx, pingCancel := context.WithTimeout(cmdCtx, 5*time.Second)
	defer pingCancel()
	_, err := rdb.Ping(pingCtx).Result()
	if err != nil {
		return fmt.Errorf("Failed to ping Redis: %w", err)
	}
	redisClient = rdb
	logger.Info("Redis connection established", "host", redisHost, "port", redisPort)
	return nil
}


