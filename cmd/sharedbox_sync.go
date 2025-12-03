package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"runtime"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/time/rate"
)

var sharedboxSyncCmd = &cobra.Command{
	Use: "sync",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if err := initAdminApiClient(); err != nil {
			return fmt.Errorf("Failed to initialize Admin API client: %v", err)
		}
		slog.DebugContext(cmd.Context(), "Initialized Admin API client")
		return nil
	},
	RunE: runSharedboxSyncCmd,
}

func init() {
	sharedboxSyncCmd.Flags().String("node", "", "Node to start syncing sharedboxes from")
	sharedboxSyncCmd.Flags().Bool("recursive", false, "Sync sharedboxes recursively")
}

func runSharedboxSyncCmd(cmd *cobra.Command, args []string) error {
	cmdCtx := cmd.Context()
	rootNode, err := cmd.Flags().GetString("node")
	if err != nil {
		return fmt.Errorf("Failed to get 'node' flag: %v", err)
	}
	recursive, _ := cmd.Flags().GetBool("recursive")
	slog.DebugContext(cmdCtx, "Starting sharedbox sync command",
		"node", rootNode,
		"recursive", recursive,
		)

	const (
		workerSize = 20
		nodeChSize = 5000
		itemChSize = 1000
		reqPerSec = 40
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

	mongoWorker := func(
		cmdCtx context.Context,
		mongoWg *sync.WaitGroup,
		itemCh <-chan SharedBoxListItemWithParent,
		errCh chan<- NodeError,
	) {
			defer mongoWg.Done()
			const (
				batchSize = 500
				flushInterval = 3 * time.Second
				reportInterval = 15 * time.Second
			)
			collection := mongoClient.Database(mongoDatabase).Collection(MONGO_COLLECTION_SHAREDBOXES)
			var (
				writeModels []mongo.WriteModel
				inserted int64
				modified int64
				matched int64
				upserted int64
			)
			flush := func() {
				if len(writeModels) == 0 {
					return
				}
				ctx, cancel := context.WithTimeout(cmdCtx, 15*time.Second)
				defer cancel()
				result, err := collection.BulkWrite(ctx, writeModels)
				if err != nil {
					errMsg := fmt.Sprintf("Bulk write error: %v", err)
					slog.ErrorContext(ctx, errMsg)
					select {
					case errCh <- NodeError{
						Err: errors.New(errMsg),
					}:
					default:
						slog.WarnContext(cmdCtx, "errCh is full, dropping error message")
					}
				} else {
					inserted += result.InsertedCount
					modified += result.ModifiedCount
					upserted += result.UpsertedCount
					matched += result.MatchedCount
				}
				writeModels = nil
			}
			ticker := time.NewTicker(flushInterval)
			reportTicker := time.NewTicker(reportInterval)
			defer ticker.Stop()
			defer reportTicker.Stop()
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
				case <-reportTicker.C:
					slog.DebugContext(
						cmdCtx, "Bulk write progress",
						"insertedCount", inserted,
						"modifiedCount", modified,
						"upsertedCount", upserted,
						"matchedCount", matched,
						)
				}
			}
		}

	redisWorker := func(
		redisWg *sync.WaitGroup,
		errCh <-chan NodeError,
	) {
			defer redisWg.Done()
			const (
				batchSize = 100
				flushInterval = 3 * time.Second
			)
			redisKey := fmt.Sprintf("%s:errors:sharedbox:sync", sessionID)
			var buffer []string
			flush := func() {
				if len(buffer) == 0 {
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), 3 * time.Second)
				defer cancel()
				err := redisClient.RPush(ctx, redisKey, buffer).Err()
				if err != nil {
					slog.ErrorContext(
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
		}

	worker := func(
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
					slog.DebugContext(ctx, "Worker context done, exiting")
					return
				case node, ok := <-nodeCh:
					if !ok {
						slog.DebugContext(ctx, "Node channel closed, exiting worker")
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
							slog.DebugContext(cmdCtx, "Fetched sharedbox list",
								"node", node,
								"count", len(resp.Lists),
								)
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
		}

	monitorWorker := func(
		ctx context.Context,
		ticker *time.Ticker,
	) {
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					slog.DebugContext(cmdCtx, "Internal status Monitor",
						"nodeCh_len", len(nodeCh),
						"nodeCh_cap", cap(nodeCh),
						"itemCh_len", len(itemCh),
						"itemCh_cap", cap(itemCh),
						"goroutines", runtime.NumGoroutine(),
						)
				}
			}
		}

	s := spinner.New(spinner.CharSets[0], 200 * time.Millisecond)
	s.FinalMSG = "sharedbox sync completed"
	s.Suffix = " Syncing sharedboxes..."
	s.Start()

	mongoWg.Add(1)
	go mongoWorker(cmdCtx, &mongoWg, itemCh, errCh)

	redisWg.Add(1)
	go redisWorker(&redisWg, errCh)

	monitorTicker := time.NewTicker(5 * time.Second)
	go monitorWorker(cmdCtx, monitorTicker)

	for i := 0; i < workerSize; i++ {
		wg.Add(1)
		go worker(cmdCtx, &wg, &jobWg, nodeCh, itemCh, errCh)
	}

	jobWg.Add(1)
	nodeCh <- rootNode

	go func() {
		jobWg.Wait()
		close(nodeCh)
	}()

	wg.Wait()

	close(itemCh)
	close(errCh)

	mongoWg.Wait()
	redisWg.Wait()

	s.Stop()

	slog.InfoContext(cmdCtx, "sharedbox sync command finished")
	return nil
}

