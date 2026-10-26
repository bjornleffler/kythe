/*
 * Copyright 2018 The Kythe Authors. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package riegeli_test

import (
	"encoding/hex"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"testing"

	"kythe.io/kythe/go/storage/stream"
	"kythe.io/kythe/go/util/compare"
	"kythe.io/kythe/go/util/riegeli"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	spb "kythe.io/kythe/proto/storage_go_proto"
	rmpb "kythe.io/third_party/riegeli/records_metadata_go_proto"
)

var (
	goldenJSONFile            = "testdata/golden.entries.json"
	goldenMetadataFile        = "testdata/golden.records_metadata.textproto"
	goldenRiegeliFilePrefix   = "testdata/golden.entries"
	goldenRiegeliFileVariants = []string{
		"uncompressed",
		"uncompressed_transpose",
		"brotli",
		"brotli_transpose",
		"snappy",
		"zstd",
	}

	// Golden files written by this package's pure-Go codecs (cgo off), read
	// back by every build. They show that a cgo build reads what a
	// cross-compiled one wrote. Regenerate with
	//   CGO_ENABLED=0 go test -run TestGeneratePureGoGolden -generate_purego_golden
	// (golden_purego_gen_test.go). Keyed by file variant; the value is the
	// RecordWriterOptions the writer records.
	goldenPureGoVariants = map[string]string{
		"purego_brotli":           "brotli",
		"purego_brotli_transpose": "brotli,transpose",
		"purego_zstd":             "zstd",
		"purego_zstd_transpose":   "transpose,zstd",
	}
)

func BenchmarkGoldenTestData(b *testing.B) {
	for _, variant := range goldenRiegeliFileVariants {
		file := strings.Join([]string{goldenRiegeliFilePrefix, variant, "riegeli"}, ".")
		b.Run(variant, func(b *testing.B) { benchGoldenData(b, file) })
	}
}

// benchGoldenData benchmarks the sequential reading of a Riegeli file.  MB/s is
// measured by the size of each record read.
func benchGoldenData(b *testing.B, goldenRiegeliFile string) {
	f, err := os.Open(goldenRiegeliFile)
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()

	b.ReportAllocs()
	rd := riegeli.NewReader(f)
	for {
		rec, err := rd.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			b.Fatal(err)
		}
		b.SetBytes(int64(len(rec)))
	}
}

type jsonReader struct{ ch <-chan *spb.Entry }

func (j *jsonReader) Next() (*spb.Entry, error) {
	e, ok := <-j.ch
	if !ok {
		return nil, io.EOF
	}
	return e, nil
}

func TestGoldenTestData(t *testing.T) {
	for _, variant := range goldenRiegeliFileVariants {
		file := strings.Join([]string{goldenRiegeliFilePrefix, variant, "riegeli"}, ".")
		opts := strings.Replace(variant, "_", ",", -1)
		t.Run(variant, func(t *testing.T) { checkGoldenData(t, file, opts) })
	}
	for variant, opts := range goldenPureGoVariants {
		file := goldenRiegeliFile(variant)
		t.Run(variant, func(t *testing.T) { checkPureGoGoldenData(t, file, opts) })
	}
}

// goldenRiegeliFile returns the path of the golden Riegeli file for variant.
func goldenRiegeliFile(variant string) string {
	return strings.Join([]string{goldenRiegeliFilePrefix, variant, "riegeli"}, ".")
}

// checkPureGoGoldenData ensures that a golden Riegeli file written by the
// pure-Go codecs holds the goldenJSONFile records. The Go writer records
// only RecordWriterOptions in its metadata, so that is all that is checked
// of it.
func checkPureGoGoldenData(t *testing.T, goldenRiegeliFile, expectedOptions string) {
	riegeliFile, err := os.Open(goldenRiegeliFile)
	if err != nil {
		t.Fatal(err)
	}
	defer riegeliFile.Close()
	riegeliReader := riegeli.NewReadSeeker(riegeliFile)

	md, err := riegeliReader.RecordsMetadata()
	if err != nil {
		t.Fatalf("Error reading RecordsMetadata: %v", err)
	} else if got := md.GetRecordWriterOptions(); got != expectedOptions {
		t.Errorf("RecordWriterOptions = %q, want %q", got, expectedOptions)
	}
	checkGoldenRecords(t, riegeliReader)
}

// checkGoldenData ensures that the given Riegeli file contains the exact
// same records as the goldenJSONFile.  It also checks the RecordsMetadata
// against goldenMetadataFile and the given expectedOptions.  RecordsMetadata
// options are the same format as the C++ strings options defined at:
// https://github.com/google/riegeli/blob/master/doc/record_writer_options.md
func checkGoldenData(t *testing.T, goldenRiegeliFile, expectedOptions string) {
	riegeliFile, err := os.Open(goldenRiegeliFile)
	if err != nil {
		t.Fatal(err)
	}
	defer riegeliFile.Close()

	riegeliReader := riegeli.NewReadSeeker(riegeliFile)

	mdTextProto, err := ioutil.ReadFile(goldenMetadataFile)
	if err != nil {
		t.Fatalf("Error reading %s: %v", goldenMetadataFile, err)
	}
	var expectedMetadata rmpb.RecordsMetadata
	if err := prototext.Unmarshal(mdTextProto, &expectedMetadata); err != nil {
		t.Fatalf("Error unmarshaling %s: %v", goldenMetadataFile, err)
	}
	expectedMetadata.RecordWriterOptions = proto.String(expectedOptions)

	md, err := riegeliReader.RecordsMetadata()
	if err != nil {
		t.Fatalf("Error reading RecordsMetadata: %v", err)
	} else if diff := compare.ProtoDiff(md, &expectedMetadata); diff != "" {
		t.Errorf("Bad RecordsMetadata: (-: found; +: expected)\n%s", diff)
	}
	checkGoldenRecords(t, riegeliReader)
}

// checkGoldenRecords ensures that riegeliReader holds exactly the
// goldenJSONFile records, reading them in order and then seeking to each.
func checkGoldenRecords(t *testing.T, riegeliReader riegeli.ReadSeeker) {
	jsonFile, err := os.Open(goldenJSONFile)
	if err != nil {
		t.Fatal(err)
	}
	defer jsonFile.Close()
	jsonReader := &jsonReader{stream.ReadJSONEntries(jsonFile)}

	var records []*spb.Entry
	var positions []riegeli.RecordPosition
	for {
		expected, err := jsonReader.Next()
		if err == io.EOF {
			if rec, err := riegeliReader.Next(); err != io.EOF {
				t.Fatalf("Unexpected error/record at end of Riegeli file: %q %v", hex.EncodeToString(rec), err)
			}
			break
		} else if err != nil {
			t.Fatalf("Error reading JSON golden data: %v", err)
		}

		pos, err := riegeliReader.Position()
		if err != nil {
			t.Fatalf("Error getting Riegeli position: %v", err)
		}
		records = append(records, expected)
		positions = append(positions, pos)

		found := &spb.Entry{}
		if err := riegeliReader.NextProto(found); err != nil {
			t.Fatalf("Error reading Riegeli golden data: %v", err)
		}

		if diff := compare.ProtoDiff(expected, found); diff != "" {
			t.Errorf("Unexpected record:  (-: found; +: expected)\n%s", diff)
		}
	}

	if rec, err := riegeliReader.Next(); err != io.EOF {
		t.Errorf("Found extra Riegeli record/error: %v %v", rec, err)
	}

	rand.New(rand.NewSource(0)).Shuffle(len(records), func(i, j int) {
		records[i], records[j] = records[j], records[i]
		positions[i], positions[j] = positions[j], positions[i]
	})

	// Seek by RecordPosition
	for i, expected := range records {
		pos := positions[i]

		if err := riegeliReader.SeekToRecord(pos); err != nil {
			t.Fatalf("Error seeking to %v: %v", pos, err)
		}
		var found spb.Entry
		if err := riegeliReader.NextProto(&found); err != nil {
			t.Fatalf("Error reading record at %v: %v", pos, err)
		} else if diff := compare.ProtoDiff(expected, &found); diff != "" {
			t.Errorf("Unexpected record:  (-: found; +: expected)\n%s", diff)
		}
	}
}
