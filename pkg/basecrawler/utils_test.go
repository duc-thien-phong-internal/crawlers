package basecrawler

import (
	"github.com/duc-thien-phong/techsharedservices/models/softwareclient"
	"testing"
)

func Test_generateAppNoFromTimeStamp(t *testing.T) {
	type args struct {
		currentTimeInMicro uint64
		softwareClientNo   softwareclient.AppNoType
	}
	tests := []struct {
		name string
		args args
		want uint64
	}{
		{
			name: "test 1",
			args: args{
				currentTimeInMicro: 1622501481147083,
				softwareClientNo:   softwareclient.ApplicationCICB,
			},
			want: 24140499617999563,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateAppNoFromTimeStamp(tt.args.currentTimeInMicro, tt.args.softwareClientNo); got != tt.want {
				t.Errorf("generateAppNoFromTimeStamp(%d) = %v, want %v", tt.args.currentTimeInMicro, got, tt.want)
			}
		})
	}
}
