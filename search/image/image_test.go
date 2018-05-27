package image

import (
	"reflect"
	"testing"
)

func TestNew(t *testing.T) {
	type want struct {
		img *Image
		err error
	}

	for _, c := range []struct {
		name string
		src  string
		want
	}{
		{
			name: "basic",
			src:  "http://www.example.com",
			want: want{
				img: &Image{ID: "http://www.example.com"},
				err: nil,
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := New(c.src)
			if err != c.want.err {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, c.want.img) {
				t.Fatalf("got %+v; want %+v", got, c.want.img)
			}
		})
	}
}
