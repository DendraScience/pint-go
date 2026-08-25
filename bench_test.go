package pint

import "testing"

func BenchmarkNewRegistry(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := NewRegistry(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	r, err := NewRegistry()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := r.Parse("3.2 kPa"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConvert(b *testing.B) {
	r, err := NewRegistry()
	if err != nil {
		b.Fatal(err)
	}
	c, err := r.Converter("degC", "degF")
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.Convert(20); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConvertN200k(b *testing.B) {
	r, err := NewRegistry()
	if err != nil {
		b.Fatal(err)
	}
	c, err := r.Converter("m", "ft")
	if err != nil {
		b.Fatal(err)
	}
	n := 200000
	src := make([]float64, n)
	dst := make([]float64, n)
	for i := range src {
		src[i] = float64(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := c.ConvertN(dst, src); err != nil {
			b.Fatal(err)
		}
	}
}
