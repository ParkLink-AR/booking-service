package trustedcaller_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/example/bookings-service/internal/controller/trustedcaller"
)

func TestTrustedCallerSuite(t *testing.T) {
	suite.Run(t, new(trustedCallerSuite))
}

type trustedCallerSuite struct {
	suite.Suite
}

type trustedCallerCase struct {
	gatewaySecret string
	internalKey   string
	headers       map[string]string
	wantStatus    int
}

var trustedCallerCases = []trustedCallerCase{
	{gatewaySecret: "gw", internalKey: "ik", headers: map[string]string{"X-Gateway-Secret": "gw"}, wantStatus: http.StatusOK},
	{gatewaySecret: "gw", internalKey: "ik", headers: map[string]string{"X-Internal-Key": "ik"}, wantStatus: http.StatusOK},
	{gatewaySecret: "gw", internalKey: "ik", headers: map[string]string{"X-User-Id": "spoofed"}, wantStatus: http.StatusUnauthorized},
	{gatewaySecret: "gw", internalKey: "ik", headers: map[string]string{"X-Gateway-Secret": "wrong"}, wantStatus: http.StatusUnauthorized},
	{gatewaySecret: "gw", internalKey: "", headers: map[string]string{"X-Internal-Key": ""}, wantStatus: http.StatusUnauthorized},
	{gatewaySecret: "", internalKey: "", headers: nil, wantStatus: http.StatusOK},
}

func (s *trustedCallerSuite) TestMiddleware() {
	for _, testCase := range trustedCallerCases {
		handler := trustedcaller.Middleware(testCase.gatewaySecret, testCase.internalKey)(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
		)
		request := httptest.NewRequest(http.MethodGet, "/bookings/x", nil)
		for key, value := range testCase.headers {
			request.Header.Set(key, value)
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		s.Equal(testCase.wantStatus, recorder.Code, "%+v", testCase)
	}
}
