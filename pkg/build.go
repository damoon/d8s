package d8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// ObjectStore manages access to a S3 compatible file store.
type ObjectStore struct {
	Client   *s3.S3
	Uploader *s3manager.Uploader
	Bucket   string
}

func (s Service) build(w http.ResponseWriter, r *http.Request) {
	chunked := r.Header.Get("d8s-chunked") != ""

	if !chunked {
		s.upstream.ServeHTTP(w, r)
		return
	}

	ctx := r.Context()

	tempfile, err := ioutil.TempFile("", "build-context-restore")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("restore context: %v", err)))
		log.Printf("restore context: %v", err)
		return
	}
	defer os.Remove(tempfile.Name())

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("read chunk list: %v", err)))
		log.Printf("read chunk list: %v", err)
		return
	}

	err = s.restoreContext(ctx, buf, tempfile)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("restore context: %v", err)))
		log.Printf("restore context: %v", err)
		return
	}

	_, err = tempfile.Seek(0, io.SeekStart)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("seek restored context file: %v", err)))
		log.Printf("seek restored context file: %v", err)
		return
	}

	r.Body = tempfile
	s.upstream.ServeHTTP(w, r)
}

func (s Service) restoreContext(ctx context.Context, chunkList io.Reader, w io.Writer) error {
	hash := make([]byte, 32)

	for {
		_, err := chunkList.Read(hash)
		if err == io.EOF {
			return nil
		}

		chunk, err := s.restoreChunk(ctx, hash)
		if err != nil {
			return err
		}

		_, err = io.Copy(w, chunk)
		if err != nil {
			return err
		}
	}
}
