package time

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseForgiving(t *testing.T) {
	type args struct {
		layout string
		value  string
	}
	tests := []struct {
		name    string
		args    args
		asIf    string
		wantErr bool
	}{
		{"pm1", args{KitchenLc, "5:06 am"}, "5:06am", false},
		{"pm2", args{KitchenLc, "5:06AM"}, "5:06am", false},
		{"pm3", args{KitchenLc, "5:06 AM"}, "5:06am", false},
		{"date1", args{UsDateTime, "4 / 3 / 2022 5:06pm"}, "4/3/2022 5:06 PM", false},
		{"err", args{UsDateTime, "abc"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseForgiving(tt.args.layout, tt.args.value)
			want, _ := time.Parse(tt.args.layout, tt.asIf)
			assert.Equal(t, err != nil, tt.wantErr)
			assert.True(t, want.Equal(got))
		})
	}
}

func TestParseInOffset(t *testing.T) {
	type args struct {
		layout   string
		value    string
		tz       string
		tzOffset int
	}
	tests := []struct {
		name    string
		args    args
		gmt     string
		wantErr bool
	}{
		{"good tz", args{UsDateTime, "6/1/2022 1:05 pm", "America/New_York", 0}, "6/1/2022 5:05 PM", false},
		{"bad tz", args{UsDateTime, "6/1/2022 1:05 pm", "bob", -240}, "6/1/2022 5:05 PM", false},
		{"blank tz", args{UsDateTime, "6/1/2022 1:05 pm", "", -240}, "6/1/2022 5:05 PM", false},
		{"err", args{UsDateTime, "abc", "", -240}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotT, err := ParseInOffset(tt.args.layout, tt.args.value, tt.args.tz, tt.args.tzOffset)
			want, _ := time.Parse(tt.args.layout, tt.gmt)

			assert.Equal(t, err != nil, tt.wantErr)
			assert.True(t, want.Equal(gotT))
		})
	}
}

func TestLayoutHasDate(t *testing.T) {
	type args struct {
		layout string
	}
	tests := []struct {
		name   string
		layout string
		want   bool
	}{
		{"with date", UsDate, true},
		{"without date", UsTime, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LayoutHasDate(tt.layout); got != tt.want {
				t.Errorf("LayoutHasDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLayoutHasTime(t *testing.T) {
	type args struct {
		layout string
	}
	tests := []struct {
		name   string
		layout string
		want   bool
	}{
		{"with time", UsTime, true},
		{"without time", UsDate, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LayoutHasTime(tt.layout); got != tt.want {
				t.Errorf("LayoutHasTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToSqlDateTime(t *testing.T) {
	s := ToSqlDateTime(NewDateTime(2000, 2, 2, 0, 0, 0, 0))
	assert.Equal(t, "2000-02-02 00:00:00.000000+00", s)

	s = ToSqlDateTime(NewDateTime(2000, 2, 2, 6, 6, 6, 0))
	assert.Equal(t, "2000-02-02 06:06:06.000000+00", s)

}
