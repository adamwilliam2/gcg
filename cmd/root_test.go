package cmd

import "testing"

func Test_toSnakeCase(t *testing.T) {
	type args struct {
		_str string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "GUID",
			args: args{
				_str: "OrderID",
			},
			want: "order_id",
		},
		{
			name: "AppVersionDomain",
			args: args{
				_str: "AppVersionDomain",
			},
			want: "app_version_domain",
		},
		{
			name: "SMS",
			args: args{
				_str: "SMS",
			},
			want: "sms",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toSnakeCase(tt.args._str); got != tt.want {
				t.Errorf("toSnakeCase() = %v, want %v", got, tt.want)
			}
		})
	}
}
