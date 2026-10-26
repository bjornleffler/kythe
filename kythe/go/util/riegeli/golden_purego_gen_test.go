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

package riegeli_test

import (
	"flag"
	"os"
	"strings"
	"testing"

	"kythe.io/kythe/go/storage/stream"
	"kythe.io/kythe/go/util/riegeli"
)

// Only in cgo-off builds, so the golden files it writes are always the
// pure-Go codecs' output.
var generatePureGoGolden = flag.Bool("generate_purego_golden", false,
	"rewrite the testdata/golden.entries.purego_*.riegeli files")

// TestGeneratePureGoGolden writes the goldenPureGoVariants files from
// goldenJSONFile. It does nothing unless -generate_purego_golden is given.
func TestGeneratePureGoGolden(t *testing.T) {
	if !*generatePureGoGolden {
		t.Skip("pass -generate_purego_golden to rewrite the pure-Go golden files")
	}
	for variant, opts := range goldenPureGoVariants {
		options, err := riegeli.ParseOptions(opts)
		if err != nil {
			t.Fatalf("ParseOptions(%q): %v", opts, err)
		}
		jsonFile, err := os.Open(goldenJSONFile)
		if err != nil {
			t.Fatal(err)
		}
		var out strings.Builder
		w := riegeli.NewWriter(&out, options)
		for e := range stream.ReadJSONEntries(jsonFile) {
			if err := w.PutProto(e); err != nil {
				t.Fatalf("PutProto: %v", err)
			}
		}
		jsonFile.Close()
		if err := w.Close(); err != nil {
			t.Fatalf("closing the %s writer: %v", variant, err)
		}
		if err := os.WriteFile(goldenRiegeliFile(variant), []byte(out.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
