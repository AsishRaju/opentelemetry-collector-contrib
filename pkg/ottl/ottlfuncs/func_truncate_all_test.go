// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_truncateAll(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("test", "hello world")
	input.PutInt("test2", 3)
	input.PutBool("test3", true)

	tests := []struct {
		name  string
		limit int64
		want  func(pcommon.Map)
	}{
		{
			name:  "truncate map",
			limit: 1,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "h")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:  "truncate map to zero",
			limit: 0,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:  "truncate nothing",
			limit: 100,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:  "truncate exact",
			limit: 11,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			setterWasCalled := false
			target := &ottl.StandardPMapGetSetter[pcommon.Map]{
				Getter: func(_ context.Context, tCtx pcommon.Map) (pcommon.Map, error) {
					return tCtx, nil
				},
				Setter: func(_ context.Context, tCtx pcommon.Map, m any) error {
					setterWasCalled = true
					if v, ok := m.(pcommon.Map); ok {
						v.CopyTo(tCtx)
						return nil
					}
					return errors.New("expected pcommon.Map")
				},
			}

			exprFunc, err := TruncateAll(target, tt.limit)
			require.NoError(t, err)

			_, err = exprFunc(nil, scenarioMap)
			require.NoError(t, err)
			assert.True(t, setterWasCalled)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_truncateAll_UTF8(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		limit  int64
		expect string
	}{
		{
			name:   "mid-rune truncation backs up to boundary",
			input:  "ab😀c", // 'ab' (2) + emoji (4) + 'c' (1) = 7 bytes
			limit:  4,      // cuts inside emoji, backs up to 'ab'
			expect: "ab",
		},
		{
			name:   "exact rune boundary preserved",
			input:  "ab😀c",
			limit:  6, // exactly after emoji
			expect: "ab😀",
		},
		{
			name:   "invalid UTF-8 uses byte-level cut",
			input:  string([]byte{0x80, 0x81, 0x82, 0x83}),
			limit:  2,
			expect: string([]byte{0x80, 0x81}),
		},
		{
			// Grapheme cluster: "👩🏾‍🦳" (woman with white hair) is 1 visible character
			// but consists of 4 Unicode code points (runes):
			//   👩 (woman)           = 4 bytes (f0 9f 91 a9)
			//   🏾 (skin tone)       = 4 bytes (f0 9f 8f be)
			//   ‍ (zero-width joiner) = 3 bytes (e2 80 8d)
			//   🦳 (white hair)      = 4 bytes (f0 9f a6 b3)
			// Total: 15 bytes, 4 runes, 1 visible character
			// Truncating at limit=10 lands in the middle of the byte 9-11,
			// so we back up to byte 8 (end of skin tone modifier).
			// Result is valid UTF-8 but a split grapheme cluster.
			name:   "grapheme cluster truncates at rune boundary not grapheme boundary",
			input:  "👩🏾‍🦳",
			limit:  10,
			expect: "👩🏾",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			scenarioMap.PutStr("test", tt.input)

			setterWasCalled := false
			target := &ottl.StandardPMapGetSetter[pcommon.Map]{
				Getter: func(_ context.Context, tCtx pcommon.Map) (pcommon.Map, error) {
					return tCtx, nil
				},
				Setter: func(_ context.Context, tCtx pcommon.Map, m any) error {
					setterWasCalled = true
					if v, ok := m.(pcommon.Map); ok {
						v.CopyTo(tCtx)
						return nil
					}
					return errors.New("expected pcommon.Map")
				},
			}

			exprFunc, err := TruncateAll(target, tt.limit)
			require.NoError(t, err)

			_, err = exprFunc(nil, scenarioMap)
			require.NoError(t, err)
			assert.True(t, setterWasCalled)

			result, _ := scenarioMap.Get("test")
			assert.Equal(t, tt.expect, result.Str())
		})
	}
}

func Test_truncateAll_validation(t *testing.T) {
	_, err := TruncateAll[any](&ottl.StandardPMapGetSetter[any]{}, -1)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid limit for truncate_all function, -1 cannot be negative")
}

func Test_truncateAll_bad_input(t *testing.T) {
	input := pcommon.NewValueStr("not a map")
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	exprFunc, err := TruncateAll[any](target, 1)
	require.NoError(t, err)

	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_truncateAll_get_nil(t *testing.T) {
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	exprFunc, err := TruncateAll[any](target, 1)
	require.NoError(t, err)

	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}

func BenchmarkTruncateAll(b *testing.B) {
	// Generate large test strings
	longASCII := strings.Repeat("hello world ", 100)                   // 1200 bytes
	longCJK := strings.Repeat("日本語テキスト", 50)                    // 900 bytes (50 * 6 chars * 3 bytes)
	longEmoji := strings.Repeat("😀😃😄😁😆", 50)                      // 1000 bytes (50 * 5 emojis * 4 bytes)
	longMixed := strings.Repeat("hello日本語😀world", 30)              // ~690 bytes
	longGrapheme := strings.Repeat("👩🏾‍🦳", 30)                        // 450 bytes (30 * 15 bytes)

	benchmarks := []struct {
		name  string
		input string
		limit int64
	}{
		// ASCII strings - various sizes
		{
			name:  "ASCII_short_10B",
			input: "hello wrld",
			limit: 5,
		},
		{
			name:  "ASCII_medium_100B",
			input: strings.Repeat("hello ", 16),
			limit: 50,
		},
		{
			name:  "ASCII_long_1KB",
			input: longASCII,
			limit: 500,
		},
		{
			name:  "ASCII_no_truncation",
			input: "hello",
			limit: 100,
		},
		// UTF-8 2-byte characters (Latin extended)
		{
			name:  "UTF8_2byte_short",
			input: "café résumé naïve",
			limit: 6,
		},
		{
			name:  "UTF8_2byte_medium",
			input: strings.Repeat("éàüö", 25),
			limit: 100,
		},
		// UTF-8 3-byte characters (CJK)
		{
			name:  "UTF8_3byte_short_9B",
			input: "日本語",
			limit: 5,
		},
		{
			name:  "UTF8_3byte_medium_90B",
			input: strings.Repeat("日本語", 10),
			limit: 45,
		},
		{
			name:  "UTF8_3byte_long_900B",
			input: longCJK,
			limit: 450,
		},
		// UTF-8 4-byte characters (emojis)
		{
			name:  "UTF8_4byte_short_20B",
			input: "ab😀cd😀ef",
			limit: 10,
		},
		{
			name:  "UTF8_4byte_medium_100B",
			input: strings.Repeat("😀😃😄😁😆", 5),
			limit: 50,
		},
		{
			name:  "UTF8_4byte_long_1KB",
			input: longEmoji,
			limit: 500,
		},
		// Mixed content
		{
			name:  "UTF8_mixed_short",
			input: "hello日本語world😀",
			limit: 15,
		},
		{
			name:  "UTF8_mixed_long_700B",
			input: longMixed,
			limit: 350,
		},
		// Grapheme clusters
		{
			name:  "UTF8_grapheme_short",
			input: "👩🏾‍🦳test",
			limit: 10,
		},
		{
			name:  "UTF8_grapheme_long_450B",
			input: longGrapheme,
			limit: 225,
		},
		// Edge cases
		{
			name:  "limit_zero",
			input: "日本語😀hello",
			limit: 0,
		},
		{
			name:  "limit_one",
			input: "日本語😀hello",
			limit: 1,
		},
		{
			name:  "invalid_UTF8_short",
			input: string([]byte{0x80, 0x81, 0x82, 0x83, 0x84, 0x85}),
			limit: 3,
		},
		{
			name:  "invalid_UTF8_long",
			input: string(bytes.Repeat([]byte{0x80, 0x81, 0x82}, 100)),
			limit: 150,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			scenarioMap := pcommon.NewMap()
			scenarioMap.PutStr("test", bm.input)

			target := &ottl.StandardPMapGetSetter[pcommon.Map]{
				Getter: func(_ context.Context, tCtx pcommon.Map) (pcommon.Map, error) {
					return tCtx, nil
				},
				Setter: func(_ context.Context, tCtx pcommon.Map, m any) error {
					if v, ok := m.(pcommon.Map); ok {
						v.CopyTo(tCtx)
						return nil
					}
					return errors.New("expected pcommon.Map")
				},
			}

			exprFunc, _ := TruncateAll(target, bm.limit)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				scenarioMap.PutStr("test", bm.input) // reset the value
				_, _ = exprFunc(nil, scenarioMap)
			}
		})
	}
}
