package cmd

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var sharedboxExportCmd = &cobra.Command{
	Use: "export",
	RunE: runSharedboxExportCmd,
}

func init() {
	sharedboxExportCmd.Flags().StringSlice("excludes", []string{}, "RootNode has this prefix will be excluded. Comma separated for multiple values.")
	sharedboxExportCmd.Flags().StringSlice("includes", []string{}, "RootNode contains this keyword will be included.")
}

func runSharedboxExportCmd(cmd *cobra.Command, args []string) error {
	cmdCtx := cmd.Context()
	excludes, _ := cmd.Flags().GetStringSlice("excludes")
	includes, _ := cmd.Flags().GetStringSlice("includes")
	defaultExcludes := os.Getenv("DEFAULT_EXPORT_EXCLUDES")
	if defaultExcludes != "" {
		subs := strings.Split(defaultExcludes, ",")
		if len(subs) > 0 {
			excludes = append(excludes, subs...)
		}
	}
	slog.DebugContext(cmdCtx, "Starting sharedbox export command",
		"excludes", excludes,
		"includes", includes,
		)

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.FinalMSG = "Sharedbox export completed."
	s.Suffix = " Exporting sharedboxes..."
	s.Start()

	collection := mongoClient.Database(mongoDatabase).Collection(MONGO_COLLECTION_SHAREDBOXES)

	cursor, err := collection.Find(
		cmdCtx,
		bson.D{},
		options.Find().SetSort(bson.D{
			{Key: "parent_node", Value: 1},
		}),
		)
	if err != nil {
		slog.ErrorContext(cmdCtx, "Failed to create MongoDB cursor for sharedboxes",
			"error", err)
		return err
	}
	defer cursor.Close(cmdCtx)

	dir := DIRECTORY_DEFAULT_EXPORT // or get from flag
	if err := os.Mkdir(dir, 0755); err != nil {
		if !os.IsExist(err) {
			slog.ErrorContext(cmdCtx, "Failed to create export directory",
				"directory", dir,)
			return err
		}
	}
	now := time.Now().In(time.Local)
	f, err := os.OpenFile(
		filepath.Join(dir, fmt.Sprintf("sharedbox_%s.csv", now.Format(time.DateTime))),
		os.O_CREATE | os.O_WRONLY | os.O_TRUNC,
		0644,
		)
	if err != nil {
		slog.ErrorContext(cmdCtx, "Failed to create export file",)
		return err
	}
	defer f.Close()
	slog.InfoContext(cmdCtx, "Exporting sharedboxes to file.",
		"file", f.Name(),
		)

	w := csv.NewWriter(f)
	defer func() {
		w.Flush()
		if err := w.Error(); err != nil {
			slog.ErrorContext(cmdCtx, "Failed to flush CSV writer",
				"error", err)
		}
	}()

	header := []string{
		"ParentNode",
		"Name",
		"Node",
		"URL",
		"DrivePath",
	}
	if err := w.Write(header); err != nil {
		slog.ErrorContext(cmdCtx, "Failed to write CSV header",)
		return err
	}

	for cursor.Next(cmdCtx) {
		var result SharedBoxListItemWithParent
		if err := cursor.Decode(&result); err != nil {
			slog.ErrorContext(cmdCtx, "Failed to decode MongoDB document",)
			continue
		}

		paths := strings.Split(result.Item.DrivePath, "/")
		if len(paths) < 3 {
			continue
		}
		rootNodePath := paths[2]
		if len(excludes) > 0 {
			exclude := false
			for _, keyword := range excludes {
				if strings.Contains(rootNodePath, keyword) {
					exclude = true
					break
				}
			}
			if exclude {
				continue
			}
		}
		if len(includes) > 0 {
			include := false
			for _, keyword := range includes {
				if strings.Contains(rootNodePath, keyword) {
					include = true
					break
				}
			}
			if !include {
				continue
			}
		}
		row := []string{
			result.ParentNode,
			result.Item.Node,
			result.Item.Name,
			result.Item.URL,
			result.Item.DrivePath,
		}
		if err := w.Write(row); err != nil {
			slog.ErrorContext(cmdCtx, "Failed to write CSV row",
				"row", row,
				)
			continue
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		slog.ErrorContext(cmdCtx, "Failed to flush CSV writer",)
		return err
	}

	s.Stop()

	slog.InfoContext(cmdCtx, "sharedbox export command finished.",)
	return nil
}
