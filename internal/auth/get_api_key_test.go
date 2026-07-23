package auth

import (
	"net/http"
	"testing"
)

type headerTest struct{
	header http.Header
	key string
	value string
	pass bool
}

func (h *headerTest) setHeaders(){
	h.header.Set(h.key, h.value)
}

func Test_APIKey(t *testing.T){
	headers := []*headerTest{
		{
			header: http.Header{},
			key: "Authorization",
			value: "ApiKey someApiKey",
			pass: true,
		},
		{
			header: http.Header{},
			key: "Something else",
			value: "ApiKey someApiKey",
			pass: false,
		},
	}

	for _, header := range headers{
		header.setHeaders()
		_, err := GetAPIKey(header.header)	
		if err == nil && header.pass == false{
			t.Error("Something is wrong")
		}else if err != nil && header.pass == true{
			t.Error("Something is wrong")
		}
	}	
}
