package storage

import (
	"fmt"
	"os"
)

// NewFromEnv returns R2Provider if R2_ACCOUNT_ID+R2_ACCESS_KEY_ID+R2_SECRET_ACCESS_KEY+R2_BUCKET_NAME are all set,
// otherwise returns LocalProvider with baseDir from UPLOAD_DIR env (default "./uploads").
func NewFromEnv() Provider {
	// Check R2 environment variables
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucketName := os.Getenv("R2_BUCKET_NAME")

	// If all R2 vars are set, try R2
	if accountID != "" && accessKeyID != "" && secretAccessKey != "" && bucketName != "" {
		r2Provider, err := NewR2Provider(accountID, accessKeyID, secretAccessKey, bucketName)
		if err != nil {
			fmt.Printf("Warning: R2 credentials provided but failed to initialize R2 provider (%v), falling back to local storage\n", err)
		} else {
			fmt.Printf("Storage: Using Cloudflare R2 (bucket: %s)\n", bucketName)
			return r2Provider
		}
	}

	// Fall back to local storage
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}

	localProvider, err := NewLocalProvider(uploadDir)
	if err != nil {
		// This should rarely fail, but if it does, panic as we need storage
		panic(fmt.Sprintf("Failed to initialize local storage provider: %v", err))
	}

	fmt.Printf("Storage: Using local disk (dir: %s)\n", uploadDir)
	return localProvider
}