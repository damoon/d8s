package wedding

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// ChunkExists returns a 200 OK in case the chunk-hash is known or a 404 Not Found in case the chunk-hash is unknown.
func (s Service) chunkExists(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")

	found, err := s.objectStore.chunkExists(r.Context(), hash)
	if err != nil {
		log.Printf("look up chunk \"%s\": %v", hash, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (o ObjectStore) chunkExists(ctx context.Context, chunkName string) (bool, error) {
	path := filepath.Join("chunks", chunkName)

	_, err := o.Client.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(o.Bucket),
		Key:    aws.String(path),
	})

	if err == nil {
		return true, nil
	}

	aerr, ok := err.(awserr.Error)
	if !ok {
		return false, fmt.Errorf("failed to cast error to awserr.Error")
	}

	switch aerr.Code() {
	case s3.ErrCodeNoSuchBucket:
		return false, fmt.Errorf("bucket %s does not exist: %v", o.Bucket, err)
	case s3.ErrCodeNoSuchKey:
		return false, nil
	case "NotFound":
		return false, nil
	}

	return false, err
}

// AddChunk stores a chunk for later reuse.
func (s Service) addChunk(w http.ResponseWriter, r *http.Request) {
	buf := bytes.NewBuffer(make([]byte, 0))
	reader := io.TeeReader(r.Body, buf)

	hash, err := hashData(reader)
	if err != nil {
		log.Printf("calculate chunk hash: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	hashHex := make([]byte, hex.EncodedLen(len(hash)))
	hex.Encode(hashHex, hash)

	log.Printf("calculated hash: %v", string(hashHex))

	path := filepath.Join("chunks", string(hashHex))

	_, err = s.objectStore.Uploader.UploadWithContext(r.Context(), &s3manager.UploadInput{
		Bucket:      aws.String(s.objectStore.Bucket),
		Key:         aws.String(path),
		ContentType: aws.String("application/octet-stream"),
		Body:        buf,
	})
	if err != nil {
		log.Printf("upload chunk to object store: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func hashData(r io.Reader) ([]byte, error) {
	h := sha256.New()

	_, err := io.Copy(h, r)
	if err != nil {
		return []byte{}, err
	}

	return h.Sum(nil), nil
}
