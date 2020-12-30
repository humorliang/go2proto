package main

import "testing"

func Test_toProtoFieldName(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "",
			args: args{
				name:"ID",
			},
			want: "id",
		},
		{
			name: "",
			args: args{
				name:"IDUser",
			},
			want: "id_user",
		},
		{
			name: "",
			args: args{
				name:"UserID",
			},
			want: "user_id",
		},
		{
			name: "",
			args: args{
				name:"userID",
			},
			want: "user_id",
		},
		{
			name: "",
			args: args{
				name:"userIdName",
			},
			want: "user_id_name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toProtoFieldName(tt.args.name); got != tt.want {
				t.Errorf("toProtoFieldName() = %v, want %v", got, tt.want)
			}
		})
	}
}