// Copyright (C) 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package analysis_test

import (
	"strings"
	"testing"

	"github.com/google/gapid/core/assert"
	"github.com/google/gapid/gapil/analysis"
	"github.com/google/gapid/core/log"
)

func TestReferenceGlobalAnalysis(t *testing.T) {
	ctx := log.Testing(t)

	common := `
    class X {
        u32   a
        Y     b
        ref!Y c
    }
    class Y {
        u32 p
    }
    ref!X G
    `

	for _, test := range []struct {
		source   string
		expected string
	}{
		{
			``,
			`<nil>`,
		}, {
			`cmd void c() { G = new!X(1, Y(2), null) }`,
			`ref!X{ a: [0x1] b: Y{ p: [0x2] } c: <nil> }`,
		}, {
			`cmd void c() { x := new!X(1, Y(2), null)  G = x  x.a = 2  x.c = new!Y(3)}`,
			`ref!X{ a: [0x2] b: Y{ p: [0x2] } c: ref!Y{ p: [0x3] } }`,
		}, {
			`cmd void c() { G = new!X(1, Y(2), new!Y(3)) }
             cmd void d() { G = new!X(2, Y(3), new!Y(4)) }`,
			`ref!X{ a: [0x1-0x2] b: Y{ p: [0x2-0x3] } c: ref!Y{ p: [0x3-0x4] } }`,
		}, {
			`cmd void c() { G = new!X(1, Y(2), new!Y(3)) }
             cmd void d() { p := G  q := p  r := q  r.a = 3 }`,
			`ref!X{ a: [0x1] [0x3] b: Y{ p: [0x2] } c: ref!Y{ p: [0x3] } }`,
		}, {
			`cmd void c() { G = null }`,
			`<nil>`,
		}, {
			`cmd void c() { if G == null { G = new!X(1, Y(2), null) } }`,
			`ref!X{ a: [0x1] b: Y{ p: [0x2] } c: <nil> }`,
		}, {
			`cmd void c() { if G != null { G = new!X(1, Y(2), null) } }`,
			`<nil>`,
		},
	} {
		ctx := ctx.S("source", test.source)
		api, mappings, err := compile(ctx, common+" "+test.source)
		assert.With(ctx).ThatError(err).Succeeded()
		res := analysis.Analyze(api, mappings)
		got := res.Globals[api.Globals[0]].(*analysis.ReferenceValue)
		s := strings.Join(strings.Fields(got.Print(res)), " ")
		assert.With(ctx).ThatString(s).Equals(test.expected)
	}
}
