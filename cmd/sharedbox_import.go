package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/time/rate"
)

var sharedboxImportCmd = &cobra.Command{
	Use: "import",
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

func runSharedboxImportCmd(cmd *cobra.Command, args []string) error {
	rootNode, err := cmd.Flags().GetString("node")
	if err != nil {
		return fmt.Errorf("Failed to get 'node' flag: %v", err)
	}
	recursive, _ := cmd.Flags().GetBool("recursive")
	logger.Info("sharedbox import", "node", rootNode, "recursive", recursive)

	cmdCtx := cmd.Context()
	const (
		workerSize = 20
		nodeChSize = 5000
		itemChSize = 1000
		reqPerSec = 20
	)
	nodeCh := make(chan string, nodeChSize)
	itemCh := make(chan SharedBoxListItemWithParent, itemChSize)
	errCh := make(chan NodeError, workerSize*2)
	var (
		wg sync.WaitGroup
		jobWg sync.WaitGroup
		mongoWg sync.WaitGroup
		redisWg sync.WaitGroup
	)
	limiter := rate.NewLimiter(rate.Limit(reqPerSec), 1)

	mongoWg.Add(1)
	go func(
		cmdCtx context.Context,
		mongoWg *sync.WaitGroup,
		itemCh <-chan SharedBoxListItemWithParent,
		errCh chan<- NodeError,
	) {
			defer mongoWg.Done()
			const (
				batchSize = 500
				flushInterval = 3 * time.Second
				defaultDatabase = "directcloud"
				defaultCollection = "sharedbox"
			)
			mongoDatabase := os.Getenv("MONGO_DATABASE")
			if mongoDatabase == "" {
				logger.Error("MONGO_DATABASE not set, using default 'sample'")
				mongoDatabase = defaultDatabase
			}
			collection := mongoClient.Database(mongoDatabase).Collection(defaultCollection)
			var writeModels []mongo.WriteModel
			flush := func() {
				if len(writeModels) == 0 {
					return
				}
				ctx, cancel := context.WithTimeout(cmdCtx, 15*time.Second)
				defer cancel()
				result, err := collection.BulkWrite(ctx, writeModels)
				if err != nil {
					errMsg := fmt.Sprintf("Bulk write error: %v", err)
					logger.ErrorContext(ctx, errMsg)
					select {
					case errCh <- NodeError{
						Err: errors.New(errMsg),
					}:
					default:
						logger.Error("errCh is full, dropping error message")
					}
				} else {
					logger.InfoContext(
						ctx, "Bulk write to MongoDB completed",
						"insertedCount", result.InsertedCount,
						"modifiedCount", result.ModifiedCount,
						"upsertedCount", result.UpsertedCount,
						"matchedCount", result.MatchedCount,
						)
				}
				writeModels = nil
			}
			ticker := time.NewTicker(flushInterval)
			defer ticker.Stop()
			for {
				select {
				case item, ok := <-itemCh:
					if !ok {
						flush()
						return
					}
					model := mongo.NewReplaceOneModel().SetFilter(bson.M{
						"item.node": item.Item.Node,
					}).SetReplacement(item).SetUpsert(true)
					writeModels = append(writeModels, model)
					if len(writeModels) >= batchSize {
						flush()
					}
				case <-ticker.C:
					flush()
				}
			}
		}(cmdCtx, &mongoWg, itemCh, errCh)

	redisWg.Add(1)
	go func(
		redisWg *sync.WaitGroup,
		errCh <-chan NodeError,
	) {
			defer redisWg.Done()
			const (
				batchSize = 100
				flushInterval = 3 * time.Second
				redisKey = "sharedbox_import_errors"
			)
			var buffer []string
			flush := func() {
				if len(buffer) == 0 {
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), 3 * time.Second)
				defer cancel()
				err := redisClient.RPush(ctx, redisKey, buffer).Err()
				if err != nil {
					logger.ErrorContext(
						ctx, "Failed to push error batch to Redis",
						"err", err,
						)
				}
				buffer = buffer[:0]
			}
			ticker := time.NewTicker(flushInterval)
			defer ticker.Stop()
			for {
				select {
				case err, ok := <-errCh:
					if !ok {
						flush()
						return
					}
					buffer = append(buffer, err.Err.Error())
					if len(buffer) >= batchSize {
						flush()
					}
				case <-ticker.C:
					flush()
				}
			}
		}(&redisWg, errCh)

	for i := 0; i < workerSize; i++ {
		wg.Add(1)
		go func(
			ctx context.Context,
			wg *sync.WaitGroup,
			jobWg *sync.WaitGroup,
			nodeCh chan string,
			itemCh chan SharedBoxListItemWithParent,
			errCh chan NodeError,
		) {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						logger.InfoContext(ctx, "Worker context done, exiting")
						return
					case node, ok := <-nodeCh:
						if !ok {
							// logger.InfoContext(ctx, "Node channel closed, exiting worker")
							return
						}
						func() {
							defer jobWg.Done()
							requestCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
							defer cancel()
							if err := limiter.Wait(requestCtx); err != nil {
								errCh <- NodeError{
									Node: node,
									Err: err,
								}
								return
							}
							resp, err := adminApiClient.SharedboxesList(requestCtx, node)
							if err != nil {
								errCh <- NodeError{
									Node: node,
									Err: err,
								}
								return
							}
							if len(resp.Lists) > 0 {
								// logger.Info(
								// 	"Fetched sharedbox list",
								// 	"node", node,
								// 	"count", len(resp.Lists),
								// 	)
								for _, item := range resp.Lists {
									itemCh <- SharedBoxListItemWithParent{
										Item: item,
										ParentNode: node,
									}
									if recursive {
										jobWg.Add(1)
										go func(ctx context.Context, node string) {
											select {
											case <-ctx.Done():
												jobWg.Done()
											case nodeCh <- node:
											}
										}(cmdCtx, item.Node)
									}
								}
							}
						}()
					}
				}
			}(cmdCtx, &wg, &jobWg, nodeCh, itemCh, errCh)
	}

	jobWg.Add(1)
	nodeCh <- rootNode

	go func() {
		jobWg.Wait()
		close(nodeCh)
	}()

	// monitorTicker := time.NewTicker(5 * time.Second)
	// go func() {
	// 	defer monitorTicker.Stop()
	// 	for {
	// 		select {
	// 		case <-monitorTicker.C:
	// 			logger.InfoContext(cmdCtx, "Internal status Monitor",
	// 				"nodeCh_len", len(nodeCh),
	// 				"nodeCh_cap", cap(nodeCh),
	// 				"itemCh_len", len(itemCh),
	// 				"itemCh_cap", cap(itemCh),
	// 				"goroutines", runtime.NumGoroutine(),
	// 				)
	// 		case <-cmdCtx.Done():
	// 			return
	// 		}
	// 	}
	// }()

	wg.Wait()

	close(itemCh)
	close(errCh)

	mongoWg.Wait()
	redisWg.Wait()

	logger.InfoContext(cmdCtx, "Sharedbox import finished successfully.")
	return nil
}

