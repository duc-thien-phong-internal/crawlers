package cicblib

import (
	"testing"

	"github.com/duc-thien-phong-internal/crawlers/libs/cicblib/mocks"
	"github.com/duc-thien-phong/techsharedservices/logger"
	"github.com/duc-thien-phong/techsharedservices/models/customer"
)

func Test_parseAddress(t *testing.T) {
	type args struct {
		address string
	}
	tests := []struct {
		name         string
		args         args
		wantCity     string
		wantDistrict string
		wantWard     string
	}{
		{
			name:         "test 1",
			args:         args{address: ".,.,HÀ BAO 2,ĐA PHƯỚC,AN PHÚ,TỈNH AN GIANG"},
			wantCity:     "TỈNH AN GIANG",
			wantDistrict: "AN PHÚ",
			wantWard:     "ĐA PHƯỚC",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCity, gotDistrict, gotWard := parseAddress(tt.args.address)
			if gotCity != tt.wantCity {
				t.Errorf("parseAddress() gotCity = %v, want %v", gotCity, tt.wantCity)
			}
			if gotDistrict != tt.wantDistrict {
				t.Errorf("parseAddress() gotDistrict = %v, want %v", gotDistrict, tt.wantDistrict)
			}
			if gotWard != tt.wantWard {
				t.Errorf("parseAddress() gotWard = %v, want %v", gotWard, tt.wantWard)
			}
		})
	}
}

func Test_parsePageToGetInformation(t *testing.T) {
	type args struct {
		htmlsource string
		app        *customer.Application
	}

	logger.InitLogger(nil)

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "test 1",
			args:    args{htmlsource: mocks.CheckWarningHTML, app: &customer.Application{}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := parsePageToGetInformation(tt.args.htmlsource, tt.args.app); (err != nil) != tt.wantErr {
				t.Errorf("parsePageToGetInformation() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.args.app.PersonalInformation.FullName != "NGUYỄN THỊ HẠNH" {
				t.Errorf("parsePageToGetInformation() fullname = %v, wantErr %v", tt.args.app.PersonalInformation.FullName, "NGUYỄN THỊ HẠNH")
			}
			if tt.args.app.PersonalInformation.Address.CurrentAddress != ".,.,HÀ BAO 2,ĐA PHƯỚC,AN PHÚ,TỈNH AN GIANG" {
				t.Errorf("parsePageToGetInformation() current address = %v, wantErr %v", tt.args.app.PersonalInformation.Address.CurrentAddress, ".,.,HÀ BAO 2,ĐA PHƯỚC,AN PHÚ,TỈNH AN GIANG")
			}
			if tt.args.app.PersonalInformation.Address.City != "TỈNH AN GIANG" {
				t.Errorf("parsePageToGetInformation() city = %v, wantErr %v", tt.args.app.PersonalInformation.Address.City, "TỈNH AN GIANG")
			}
			if tt.args.app.PersonalInformation.Address.District != "AN PHÚ" {
				t.Errorf("parsePageToGetInformation() DISTRICT = %v, wantErr %v", tt.args.app.PersonalInformation.Address.District, "AN PHÚ")
			}
			if tt.args.app.PersonalInformation.Address.Ward != "ĐA PHƯỚC" {
				t.Errorf("parsePageToGetInformation() Ward = %v, wantErr %v", tt.args.app.PersonalInformation.Address.Ward, "ĐA PHƯỚC")
			}
			if tt.args.app.PersonalInformation.IDCardNumber != "351938970" {
				t.Errorf("parsePageToGetInformation() idcard number = %v, wantErr %v", tt.args.app.PersonalInformation.IDCardNumber, "351938970")
			}
			if tt.args.app.CurrentCICCode != "8931915515" {
				t.Errorf("parsePageToGetInformation() current cic code = %v, wantErr %v", tt.args.app.CurrentCICCode, "8931915515")
			}
			if tt.args.app.PersonalInformation.Mobile != "01234995507" {
				t.Errorf("parsePageToGetInformation() current mobile number = %v, wantErr %v", tt.args.app.PersonalInformation.Mobile, "01234995507")
			}
			if tt.args.app.CurrentCICInfo != "Khách hàng hiện không có quan hệ tại TCTD, không có nợ cần chú ý và không có nợ xấu tại thời điểm cuối tháng 31/03/2021" {
				t.Errorf("parsePageToGetInformation() current cic info = `%v`, wantErr `%v`", tt.args.app.CurrentCICInfo, "Khách hàng hiện không có quan hệ tại TCTD, không có nợ cần chú ý và không có nợ xấu tại thời điểm cuối tháng 31/03/2021")
			}
		})
	}
}
