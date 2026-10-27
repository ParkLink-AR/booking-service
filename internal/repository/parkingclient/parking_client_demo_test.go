package parkingclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/example/bookings-service/internal/repository/parkingclient"
)

func TestListDemoSpotsSuite(t *testing.T) {
	suite.Run(t, new(listDemoSpotsSuite))
}

type listDemoSpotsSuite struct{ suite.Suite }

func (s *listDemoSpotsSuite) TestReadsTheDemoSpotsFromTheInternalRouteWithTheInternalKey() {
	var path, key string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path, key = request.URL.Path, request.Header.Get("X-Internal-Key")
		_, _ = writer.Write([]byte(`[{"id":"spot-1","ownerId":"owner-1","pricePerHour":7500},{"id":"spot-2","ownerId":"owner-2","pricePerHour":null}]`))
	}))
	defer server.Close()

	spots, err := parkingclient.NewClient(server.URL, "secret-key").ListDemoSpots(context.Background())

	s.Require().NoError(err)
	s.Equal("/internal/demo-spots", path)
	s.Equal("secret-key", key)
	s.Require().Len(spots, 2)
	s.Equal("spot-1", spots[0].ID)
	s.Equal("owner-1", spots[0].OwnerID)
	s.Require().NotNil(spots[0].PricePerHour)
	s.Equal(7500.0, *spots[0].PricePerHour)
	s.Nil(spots[1].PricePerHour)
}

func (s *listDemoSpotsSuite) TestAnEmptyListIsNotAnError() {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`[]`))
	}))
	defer server.Close()

	spots, err := parkingclient.NewClient(server.URL, "k").ListDemoSpots(context.Background())

	s.Require().NoError(err)
	s.Empty(spots)
}

func (s *listDemoSpotsSuite) TestAFailureStatusIsAnError() {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := parkingclient.NewClient(server.URL, "k").ListDemoSpots(context.Background())

	s.Require().Error(err)
}

func (s *listDemoSpotsSuite) TestABrokenBodyIsAnError() {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`not json`))
	}))
	defer server.Close()

	_, err := parkingclient.NewClient(server.URL, "k").ListDemoSpots(context.Background())

	s.Require().Error(err)
}

func (s *listDemoSpotsSuite) TestParkingBeingUnreachableIsAnError() {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()

	_, err := parkingclient.NewClient(url, "k").ListDemoSpots(context.Background())

	s.Require().Error(err)
}
