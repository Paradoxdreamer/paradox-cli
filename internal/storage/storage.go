package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/paradox-cloud/paradox/internal/logging"
	"github.com/paradox-cloud/paradox/internal/registry"
	"github.com/spf13/cobra"
)

func init() {
	registry.Register(&registry.Service{
		Name:          "storage",
		Description:   "S3-style object storage — buckets, objects, signed URLs",
		ConfigSection: "storage",
		Commands:      []*cobra.Command{storageCmd},
		DoctorChecks: []registry.DoctorCheck{
			{Name: "Storage store", Fn: checkStorageStore},
		},
	})

	storageCmd.AddCommand(storageInitCmd)
	storageCmd.AddCommand(storageMbCmd)
	storageCmd.AddCommand(storageRbCmd)
	storageCmd.AddCommand(storageLsCmd)
	storageCmd.AddCommand(storagePutCmd)
	storageCmd.AddCommand(storageGetCmd)
	storageCmd.AddCommand(storageRmCmd)
	storageCmd.AddCommand(storageStatCmd)
	storageCmd.AddCommand(storageSignCmd)
}

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Paradox Storage — S3-style object store",
	Long: `Local S3-compatible object storage.

  paradox storage init
  paradox storage mb <bucket>
  paradox storage rb <bucket>
  paradox storage ls [bucket] [prefix]
  paradox storage put <bucket> <key> -f file
  paradox storage get <bucket> <key> [-o file]
  paradox storage rm <bucket> <key>
  paradox storage stat <bucket> <key>
  paradox storage sign <bucket> <key> [--ttl 1h]
`,
}

var storageInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the storage store",
	RunE: func(cmd *cobra.Command, args []string) error {
		root, err := storageRoot()
		if err != nil {
			return err
		}
		if _, err := NewStore(root); err != nil {
			return err
		}
		fmt.Printf("✓ Storage ready at %s\n", root)
		logging.Info("storage initialized", "path", root)
		return nil
	},
}

var storageMbCmd = &cobra.Command{
	Use:   "mb <bucket>",
	Short: "Create a bucket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		if err := store.CreateBucket(args[0]); err != nil {
			return err
		}
		fmt.Printf("✓ Bucket %q created\n", sanitizeBucket(args[0]))
		return nil
	},
}

var storageRbCmd = &cobra.Command{
	Use:   "rb <bucket>",
	Short: "Remove an empty bucket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		if err := store.DeleteBucket(args[0]); err != nil {
			return err
		}
		fmt.Printf("✓ Bucket %q removed\n", sanitizeBucket(args[0]))
		return nil
	},
}

var storageLsCmd = &cobra.Command{
	Use:   "ls [bucket] [prefix]",
	Short: "List buckets or objects",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		if len(args) == 0 {
			buckets, err := store.ListBuckets()
			if err != nil {
				return err
			}
			if len(buckets) == 0 {
				fmt.Println("(no buckets)")
				return nil
			}
			for _, b := range buckets {
				fmt.Printf("  %s  %s\n", b.CreatedAt.Format(time.RFC3339), b.Name)
			}
			return nil
		}
		prefix := ""
		if len(args) > 1 {
			prefix = args[1]
		}
		objs, err := store.ListObjects(args[0], prefix)
		if err != nil {
			return err
		}
		if len(objs) == 0 {
			fmt.Println("(no objects)")
			return nil
		}
		fmt.Printf("%-12s %10s  %s\n", "UPLOADED", "SIZE", "KEY")
		for _, o := range objs {
			fmt.Printf("%-12s %10d  %s\n", o.UploadedAt.Format("2006-01-02"), o.Size, o.Key)
		}
		return nil
	},
}

var (
	putFile        string
	putContentType string
)

var storagePutCmd = &cobra.Command{
	Use:   "put <bucket> <key>",
	Short: "Upload a file",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if putFile == "" {
			return fmt.Errorf("--file is required")
		}
		store, err := openStore()
		if err != nil {
			return err
		}
		f, err := os.Open(putFile)
		if err != nil {
			return err
		}
		defer f.Close()
		ct := putContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		om, err := store.Put(args[0], args[1], f, ct, nil)
		if err != nil {
			return err
		}
		fmt.Printf("✓ Uploaded s3://%s/%s (%d bytes, etag=%s)\n", om.Bucket, om.Key, om.Size, om.ETag)
		logging.Info("object put", "bucket", om.Bucket, "key", om.Key, "size", om.Size)
		return nil
	},
}

var getOut string

var storageGetCmd = &cobra.Command{
	Use:   "get <bucket> <key>",
	Short: "Download an object",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		om, r, err := store.Get(args[0], args[1])
		if err != nil {
			return err
		}
		defer r.Close()
		out := getOut
		if out == "" {
			out = filepath.Base(om.Key)
		}
		f, err := os.Create(out)
		if err != nil {
			return err
		}
		defer f.Close()
		n, err := io.Copy(f, r)
		if err != nil {
			return err
		}
		fmt.Printf("✓ Downloaded %s (%d bytes)\n", out, n)
		return nil
	},
}

var storageRmCmd = &cobra.Command{
	Use:   "rm <bucket> <key>",
	Short: "Delete an object",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		if err := store.Delete(args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("✓ Deleted s3://%s/%s\n", sanitizeBucket(args[0]), sanitizeKey(args[1]))
		return nil
	},
}

var storageStatCmd = &cobra.Command{
	Use:   "stat <bucket> <key>",
	Short: "Show object metadata",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		om, err := store.Head(args[0], args[1])
		if err != nil {
			return err
		}
		fmt.Printf("  bucket:       %s\n", om.Bucket)
		fmt.Printf("  key:          %s\n", om.Key)
		fmt.Printf("  size:         %d\n", om.Size)
		fmt.Printf("  content_type: %s\n", om.ContentType)
		fmt.Printf("  etag:         %s\n", om.ETag)
		fmt.Printf("  uploaded_at:  %s\n", om.UploadedAt.Format(time.RFC3339))
		return nil
	},
}

var signTTL string

var storageSignCmd = &cobra.Command{
	Use:   "sign <bucket> <key>",
	Short: "Generate a signed URL",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openStore()
		if err != nil {
			return err
		}
		ttl := time.Hour
		if signTTL != "" {
			d, err := time.ParseDuration(signTTL)
			if err != nil {
				return fmt.Errorf("invalid --ttl: %w", err)
			}
			ttl = d
		}
		url, err := store.SignedURL(args[0], args[1], "GET", ttl)
		if err != nil {
			return err
		}
		fmt.Println(url)
		return nil
	},
}

func init() {
	storagePutCmd.Flags().StringVarP(&putFile, "file", "f", "", "local file to upload")
	storagePutCmd.Flags().StringVar(&putContentType, "content-type", "", "content type")
	_ = storagePutCmd.MarkFlagRequired("file")

	storageGetCmd.Flags().StringVarP(&getOut, "out", "o", "", "output file (default: basename of key)")

	storageSignCmd.Flags().StringVar(&signTTL, "ttl", "1h", "URL lifetime")
}

func openStore() (*Store, error) {
	root, err := storageRoot()
	if err != nil {
		return nil, err
	}
	return NewStore(root)
}

func checkStorageStore() (ok bool, detail string) {
	root, err := storageRoot()
	if err != nil {
		return false, err.Error()
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "not initialized (run `paradox storage init`)"
		}
		return false, err.Error()
	}
	if !info.IsDir() {
		return false, root + " is not a directory"
	}
	return true, root
}
