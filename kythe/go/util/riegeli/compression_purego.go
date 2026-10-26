//go:build !cgo

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

// The brotli and zstd codecs in pure Go, used when cgo is not available,
// such as a cross-compiled build.

import (
	"fmt"
	"io"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// zstdDecoder is shared by every block. DecodeAll is safe for concurrent
// use, and one decoder avoids allocating its state per block.
var zstdDecoder, zstdDecoderErr = zstd.NewReader(nil)

// zstdEncoders holds one *zstd.Encoder per klauspost encoder level, created
// on first use. Each is limited to one concurrent encode, as an encoder's
// state is large and riegeli writes are not the hot path.
var zstdEncoders sync.Map

// decompressBrotli decodes a brotli stream holding size bytes.
func decompressBrotli(r io.Reader, size uint64) ([]byte, error) {
	return readDecoded(io.NopCloser(brotli.NewReader(r)), size)
}

// decompressZstd decodes a zstd stream holding size bytes. The stream is
// the rest of r, as for snappy.
func decompressZstd(r io.Reader, size uint64) ([]byte, error) {
	if zstdDecoderErr != nil {
		return nil, zstdDecoderErr
	}
	compressed, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	decoded, err := zstdDecoder.DecodeAll(compressed, make([]byte, 0, size))
	if err != nil {
		return nil, err
	}
	if uint64(len(decoded)) != size {
		return nil, fmt.Errorf("zstd block decoded to %d bytes; expected %d", len(decoded), size)
	}
	return decoded, nil
}

// newBrotliWriter returns a brotli encoder at the given quality writing to w.
func newBrotliWriter(w io.Writer, quality int) io.WriteCloser {
	return brotli.NewWriterOptions(w, brotli.WriterOptions{Quality: quality})
}

// compressZstd encodes src as a single zstd frame. klauspost has four
// encoder levels rather than zstd's 22, so level picks the nearest one.
func compressZstd(src []byte, level int) ([]byte, error) {
	speed := zstd.EncoderLevelFromZstd(level)
	encoder, ok := zstdEncoders.Load(speed)
	if !ok {
		created, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(speed), zstd.WithEncoderConcurrency(1))
		if err != nil {
			return nil, err
		}
		var loaded bool
		if encoder, loaded = zstdEncoders.LoadOrStore(speed, created); loaded {
			created.Close()
		}
	}
	return encoder.(*zstd.Encoder).EncodeAll(src, nil), nil
}
