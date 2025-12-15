package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	sessionID string
	logFile   *os.File
	mongoClient *mongo.Client
	redisClient *redis.Client
	adminApiClient *AdminApiClient
	mongoDatabase string
)
const (
	MONGO_DEFAULT_DATABASE = "amazing_brain_dead_storage_accessor"
	MONGO_COLLECTION_SHAREDBOXES = "sharedboxes"
	MONGO_COLLECTION_USERS = "users"
	DIRECTORY_DEFAULT_LOGS = "logs"
	DIRECTORY_DEFAULT_EXPORT = "exports"
)

var rootCmd = &cobra.Command{
	Use:   "amazing-brain-dead-storage-accessor",
	PersistentPreRunE: func (cmd *cobra.Command, args []string) error {
		_ = godotenv.Load()
		initSessionID()
		if err := initLogger(cmd); err != nil {
			return err
		}
		if err := initMongoClient(cmd); err != nil {
			return err
		}
		if err := initRedisClient(cmd); err != nil {
			return err
		}
		if err := initMongoIndexes(cmd); err != nil {
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
				combinedErr = errors.Join(combinedErr, err)
			}
		}
		if redisClient != nil {
			if err := redisClient.Close(); err != nil {
				log.Printf("Error closing Redis client: %v", err)
				combinedErr = errors.Join(combinedErr, err)
			}
		}
		if logFile != nil {
			if err := logFile.Close(); err != nil {
				log.Printf("Error closing log file: %v", err)
				combinedErr = errors.Join(combinedErr, err)
			}
		}
		return combinedErr
	},
}

func init() {
	rootCmd.PersistentFlags().Bool("verbose", false, "Enable verbose output")
	rootCmd.AddCommand(
		authCmd,
		userCmd,
		sharedboxCmd,
		)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("Command execution failed", "error", err)
		os.Exit(1)
	}
}

func initSessionID() {
	sessionID = uuid.New().String()
}

func initLogger(cmd *cobra.Command) error {
	cmdCtx := cmd.Context()
	logLevel := slog.LevelInfo
	verbose, _ := cmd.Flags().GetBool("verbose")
	if verbose {
		logLevel = slog.LevelDebug
	}
	logDir := "logs"
	if err := os.Mkdir(logDir, 0755); err != nil {
		if !os.IsExist(err) {
			slog.ErrorContext(cmdCtx, "Failed to create log directory",
				"error", err,
				)
			return err
		}
	}
	now := time.Now().In(time.Local).Format(time.DateOnly)
	logFile, err := os.OpenFile(
		filepath.Join(logDir, fmt.Sprintf("debug-%s.log", now)),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
		)
	if err != nil {
		slog.ErrorContext(cmdCtx, "Failed to open log file",
			"error", err,
			)
		return err
	}
	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: logLevel,
		AddSource: false,
	})).With(
		slog.String("sessionID", sessionID),
		)
	slog.SetDefault(logger)
	slog.InfoContext(cmdCtx, "-------------------------")
	slog.InfoContext(cmdCtx, "Application started",)
	return nil
}

func initMongoClient(cmd *cobra.Command) error {
	cmdCtx := cmd.Context()
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
	mongoDatabase = os.Getenv("MONGO_DATABASE")
	if mongoDatabase == "" {
		mongoDatabase = MONGO_DEFAULT_DATABASE
	}
	slog.DebugContext(cmdCtx, "MongoDB connection established",
		"host", mongoHost,
		"port", mongoPort,
		"database", mongoDatabase,
		)
	return nil
}
func initMongoIndexes(cmd *cobra.Command) error {
	cmdCtx := cmd.Context()
	slog.DebugContext(cmdCtx, "Using MongoDB database",
		"database", mongoDatabase,
		)
	db := mongoClient.Database(mongoDatabase)
	var indexErr error
	func() {
		collection := db.Collection(MONGO_COLLECTION_SHAREDBOXES)
		ctx, cancel := context.WithTimeout(cmdCtx, 30*time.Second)
		defer cancel()
		_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{
				{Key: "item.node", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("idx_node_unique"),
		})
		if err != nil {
			slog.ErrorContext(cmdCtx, "Failed to create index on sharedboxes collection",
				"error", err,
				)
			indexErr = errors.Join(indexErr, err)
			return
		}
		slog.DebugContext(cmdCtx, "MongoDB index ensured on sharedboxes collection",)
	}()
	func() {
		collection := db.Collection(MONGO_COLLECTION_USERS)
		ctx, cancel := context.WithTimeout(cmdCtx, 30*time.Second)
		defer cancel()
		_, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys: bson.D{
				{Key: "user_seq", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("idx_user_seq_unique"),
		})
		if err != nil {
			slog.ErrorContext(cmdCtx, "Failed to create index on sharedboxes collection",
				"error", err,
				)
			indexErr = errors.Join(indexErr, err)
			return
		}
		slog.DebugContext(cmdCtx, "MongoDB index ensured on users collection",)
	}()
	return indexErr
}
func closeMongoClient(cmdCtx context.Context) error {
	disconnectCtx, disconnectCancel := context.WithTimeout(cmdCtx, 10*time.Second)
	defer disconnectCancel()
	if err := mongoClient.Disconnect(disconnectCtx); err != nil {
		return fmt.Errorf("Failed to disconnect from MongoDB: %v", err)
	}
	return nil
}
func initRedisClient(cmd *cobra.Command) error {
	cmdCtx := cmd.Context()
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
	slog.DebugContext(cmdCtx, "Redis connection established",
		"host", redisHost,
		"port", redisPort,
		)
	return nil
}

