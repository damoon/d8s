package d8s

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"
)

func Test_digestParser_publish(t *testing.T) {
	type fields struct {
		buf bytes.Buffer
		w   io.Writer
	}
	tests := []struct {
		name    string
		fields  fields
		input   string
		wantW   string
		wantErr bool
	}{
		{
			name: "found",
			fields: fields{
				buf: bytes.Buffer{},
				w:   ioutil.Discard,
			},
			input: `#5 [2/2] RUN sleep 1
#5 DONE 1.2s

#7 exporting to image
#7 exporting layers
#7 exporting layers 0.4s done
#7 exporting manifest sha256:d8438874a02b14e2ad7be50f7505ec3d9fe645964e6987101179ef42f8bed5b6 0.0s done
#7 exporting config sha256:0d3cc5d5b92a708fbabb79b63b59839ca87012742a1b5e741cf51fd1ad14b804 0.0s done
#7 pushing layers
#7 pushing layers 0.2s done
#7 pushing manifest for wedding-registry:5000/digests:latest
#7 pushing manifest for wedding-registry:5000/digests:latest 0.1s done
#7 DONE 0.7s

#8 exporting cache
#8 preparing build cache for export 0.1s done
#8 writing layer sha256:166a2418f7e86fa48d87bf6807b4e5b35f078acb2ad1cbf10444a7025913c24f
#8 writing layer sha256:166a2418f7e86fa48d87bf6807b4e5b35f078acb2ad1cbf10444a7025913c24f done
#8 writing layer sha256:1966ea362d2394e7c5c508ebf3695f039dd3825bd1e7a07449ae530aea3c4cd1 done
#8 writing layer sha256:5a9f1c0027a73bc0e66a469f90e47a59e23ab3472126ed28e6a4e7b1a98d1eb5 done
#8 writing layer sha256:b5c20b2b484f5ca9bc9d98dc79f8f1381ee0c063111ea0ddf42d1ae5ea942d50 done
#8 writing layer sha256:bb79b6b2107fea8e8a47133a660b78e3a546998fcf0427be39ac9a0af4a97e90 done
#8 writing layer sha256:e2eabaeb95d9574853154f705dc7ebce6184a95cd153e3ff87108e200267aa0a 0.1s done
#8 writing config sha256:d7e07a2c74d972a2a645ea9ba4d4970a71b2795611236ff61810cb30b60d7725 0.1s done
#8 writing manifest sha256:3acb5d16e32a8cf7094e195a5d24ca15d4fbe8a433a8bd5cc2365040739eb2dc`,
			wantW: `{"aux":{"ID":"sha256:d8438874a02b14e2ad7be50f7505ec3d9fe645964e6987101179ef42f8bed5b6"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := digestParser{
				buf: tt.fields.buf,
				w:   tt.fields.w,
			}

			if _, err := d.Write([]byte(tt.input)); err != nil {
				t.Errorf("digestParser.Write() error = %v", err)
				return
			}

			w := &bytes.Buffer{}
			if err := d.publish(w); (err != nil) != tt.wantErr {
				t.Errorf("digestParser.publish() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotW := w.String(); gotW != tt.wantW {
				t.Errorf("digestParser.publish() = %v, want %v", gotW, tt.wantW)
			}
		})
	}
}
