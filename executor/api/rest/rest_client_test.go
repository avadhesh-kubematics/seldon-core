package rest

import (
	"context"
	"crypto/tls"
	"github.com/golang/protobuf/jsonpb"
	"github.com/onsi/gomega"
	"github.com/seldonio/seldon-core/executor/api/grpc/proto"
	"github.com/seldonio/seldon-core/executor/api/payload"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

const (
	okPredictResponse = `{
		"data": {
           "names" : [ "a", "b" ],
           "ndarray" : [[0.9,0.1]]
       }
	}`
	okRouteResponse = `{
		"data": {
           "ndarray" : [1]
       }
	}`
)

func testingHTTPClient(g *gomega.GomegaWithT, handler http.Handler) (string, int, *http.Client, func()) {
	s := httptest.NewServer(handler)

	cli := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, network, _ string) (net.Conn, error) {
				return net.Dial(network, s.Listener.Addr().String())
			},
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	url, err := url.Parse(s.URL)
	g.Expect(err).Should(gomega.BeNil())
	port, err := strconv.Atoi(url.Port())
	g.Expect(err).Should(gomega.BeNil())

	return url.Hostname(), port, cli, s.Close
}

func SetHTTPClient(httpClient *http.Client) BytesRestClientOption {
	return func(cli *JSONRestClient) {
		cli.httpClient = httpClient
	}
}

func createPayload(g *gomega.GomegaWithT) payload.SeldonPayload {
	var data = ` {"data":{"ndarray":[1.1,2.0]}}`
	return &payload.BytesPayload{Msg: []byte(data)}
}

func TestSimpleMethods(t *testing.T) {
	t.Logf("Started")
	g := gomega.NewGomegaWithT(t)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okPredictResponse))
	})
	host, port, httpClient, teardown := testingHTTPClient(g, h)
	defer teardown()
	seldonRestClient := NewJSONRestClient(SetHTTPClient(httpClient))

	methods := []func(context.Context, string, int32, payload.SeldonPayload) (payload.SeldonPayload, error){seldonRestClient.Predict, seldonRestClient.TransformInput, seldonRestClient.TransformOutput}
	for _, method := range methods {
		resPayload, err := method(context.TODO(), host, int32(port), createPayload(g))
		g.Expect(err).Should(gomega.BeNil())

		data := resPayload.GetPayload().([]byte)
		var smRes proto.SeldonMessage
		err = jsonpb.UnmarshalString(string(data), &smRes)
		g.Expect(err).Should(gomega.BeNil())
		g.Expect(smRes.GetData().GetNdarray().Values[0].GetListValue().Values[0].GetNumberValue()).Should(gomega.Equal(0.9))
		g.Expect(smRes.GetData().GetNdarray().Values[0].GetListValue().Values[1].GetNumberValue()).Should(gomega.Equal(0.1))
	}

}

func TestRouter(t *testing.T) {
	t.Logf("Started")
	g := gomega.NewGomegaWithT(t)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okRouteResponse))
	})
	host, port, httpClient, teardown := testingHTTPClient(g, h)
	defer teardown()
	seldonRestClient := NewJSONRestClient(SetHTTPClient(httpClient))

	route, err := seldonRestClient.Route(context.TODO(), host, int32(port), createPayload(g))
	g.Expect(err).Should(gomega.BeNil())

	g.Expect(route).Should(gomega.Equal(1))
}
func createCombinerPayload(g *gomega.GomegaWithT) []payload.SeldonPayload {
	var data = ` {"data":{"ndarray":[1.1,2.0]}}`
	smp := []payload.SeldonPayload{&payload.BytesPayload{Msg: []byte(data)}, &payload.BytesPayload{Msg: []byte(data)}}
	return smp
}

func TestCombiner(t *testing.T) {
	t.Logf("Started")
	g := gomega.NewGomegaWithT(t)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(okPredictResponse))
	})
	host, port, httpClient, teardown := testingHTTPClient(g, h)
	defer teardown()
	seldonRestClient := NewJSONRestClient(SetHTTPClient(httpClient))

	resPayload, err := seldonRestClient.Combine(context.TODO(), host, int32(port), createCombinerPayload(g))
	g.Expect(err).Should(gomega.BeNil())

	data := resPayload.GetPayload().([]byte)
	var smRes proto.SeldonMessage
	err = jsonpb.UnmarshalString(string(data), &smRes)
	g.Expect(err).Should(gomega.BeNil())
	g.Expect(smRes.GetData().GetNdarray().Values[0].GetListValue().Values[0].GetNumberValue()).Should(gomega.Equal(0.9))
	g.Expect(smRes.GetData().GetNdarray().Values[0].GetListValue().Values[1].GetNumberValue()).Should(gomega.Equal(0.1))
}
