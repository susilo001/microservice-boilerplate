package request

import (
	"testing"

	"github.com/gin-gonic/gin/binding"
)

func TestCreateProductRequestBinding(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "valid request", body: `{"name":"Keyboard","price":19.99,"stock":0}`},
		{name: "missing name", body: `{"price":19.99,"stock":0}`, wantErr: true},
		{name: "non-positive price", body: `{"name":"Keyboard","price":0,"stock":0}`, wantErr: true},
		{name: "negative stock", body: `{"name":"Keyboard","price":19.99,"stock":-1}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var request CreateProductRequest
			err := binding.JSON.BindBody([]byte(tt.body), &request)
			if (err != nil) != tt.wantErr {
				t.Fatalf("BindBody() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
