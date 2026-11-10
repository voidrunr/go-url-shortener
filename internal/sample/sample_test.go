package sample

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSampleReset(t *testing.T) {
	str := "payload"
	when := time.Now()
	s := &Sample{
		i:     42,
		s:     "text",
		nu:    7,
		sp:    &str,
		f:     1.5,
		b:     true,
		items: []string{"a", "b", "c"},
		m:     map[string]string{"k": "v"},
		t:     when,
		tp:    &when,
		child: &Sample{i: 9, s: "inner"},
		other: &Sub{n: 5, txt: "deep"},
	}
	s.items[0] = "filled"

	s.Reset()

	require.Equal(t, 0, s.i)
	require.Empty(t, s.s)
	require.Equal(t, Number(0), s.nu)
	require.NotNil(t, s.sp)
	require.Empty(t, *s.sp)
	require.Zero(t, s.f)
	require.False(t, s.b)

	require.Len(t, s.items, 0)
	require.Equal(t, 3, cap(s.items), "slice must be truncated, not reallocated")
	require.NotNil(t, s.m)
	require.Len(t, s.m, 0)

	require.True(t, s.t.IsZero())
	require.NotNil(t, s.tp)
	require.True(t, s.tp.IsZero())

	require.Equal(t, 0, s.child.i)
	require.Empty(t, s.child.s)
	require.Equal(t, 0, s.other.n)
	require.Empty(t, s.other.txt)
}

func TestSubReset(t *testing.T) {
	sub := &Sub{n: 3, txt: "hi"}
	sub.Reset()
	require.Equal(t, 0, sub.n)
	require.Empty(t, sub.txt)
}

func TestResetNilReceiver(t *testing.T) {
	var str *string
	var when *time.Time
	s := &Sample{sp: str, tp: when, child: nil, other: nil}

	require.NotPanics(t, func() { s.Reset() })
	(&Sub{}).Reset()

	var nilSample *Sample
	require.NotPanics(t, func() { nilSample.Reset() })
}
