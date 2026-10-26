//go:build cgo

/*
 * Copyright 2026 The Kythe Authors. All rights reserved.
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

package riegeli

// The brotli and zstd codecs backed by the C libraries, used whenever cgo
// is available.

import (
	"io"

	"github.com/DataDog/zstd"
	"github.com/google/brotli/go/cbrotli"
)

// decompressBrotli decodes a brotli stream holding size bytes.
func decompressBrotli(r io.Reader, size uint64) ([]byte, error) {
	return readDecoded(cbrotli.NewReader(r), size)
}

// decompressZstd decodes a zstd stream holding size bytes.
func decompressZstd(r io.Reader, size uint64) ([]byte, error) {
	return readDecoded(zstd.NewReader(r), size)
}

// newBrotliWriter returns a brotli encoder at the given quality writing to w.
func newBrotliWriter(w io.Writer, quality int) io.WriteCloser {
	return cbrotli.NewWriter(w, cbrotli.WriterOptions{Quality: quality})
}

// compressZstd encodes src as a single zstd frame at the given level.
func compressZstd(src []byte, level int) ([]byte, error) {
	return zstd.CompressLevel(nil, src, level)
}
