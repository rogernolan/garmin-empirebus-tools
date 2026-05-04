package teltonika

import (
	"encoding/json"
	"testing"
)

func TestCoordinatesFromPayloadFindsNestedStatusFields(t *testing.T) {
	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"gps": map[string]interface{}{
				"latitude":  "51.5007",
				"longitude": -0.1246,
			},
		},
	}
	lat, lon, err := coordinatesFromPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if lat != 51.5007 || lon != -0.1246 {
		t.Fatalf("got %f,%f", lat, lon)
	}
}

func TestCoordinatesFromPayloadRejectsMissingLongitude(t *testing.T) {
	_, _, err := coordinatesFromPayload(map[string]interface{}{"latitude": 51.5})
	if err == nil {
		t.Fatal("expected missing longitude error")
	}
}

func TestTokenFromPayloadFindsNestedToken(t *testing.T) {
	payload := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"token": "abc123",
		},
	}
	token, ok := tokenFromPayload(payload)
	if !ok {
		t.Fatal("expected token")
	}
	if token != "abc123" {
		t.Fatalf("got token %q", token)
	}
}

func TestLoginRequestBodyUsesRutOSDataEnvelope(t *testing.T) {
	var got map[string]map[string]string
	bodies, err := loginRequestBodies("jones", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 {
		t.Fatalf("got %d bodies", len(bodies))
	}
	if err := json.Unmarshal(bodies[0], &got); err != nil {
		t.Fatal(err)
	}
	if got["data"]["username"] != "jones" {
		t.Fatalf("got username %q", got["data"]["username"])
	}
	if got["data"]["password"] != "secret" {
		t.Fatalf("got password %q", got["data"]["password"])
	}
}

func TestLoginRequestBodiesIncludeLegacyTopLevelPayload(t *testing.T) {
	var got map[string]string
	bodies, err := loginRequestBodies("jones", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(bodies[1], &got); err != nil {
		t.Fatal(err)
	}
	if got["username"] != "jones" {
		t.Fatalf("got username %q", got["username"])
	}
	if got["password"] != "secret" {
		t.Fatalf("got password %q", got["password"])
	}
}

func TestEndpointPathDefaultsToGPSPositionStatus(t *testing.T) {
	if got := endpointPath(""); got != "/api/gps/position/status" {
		t.Fatalf("got path %q", got)
	}
}

func TestEndpointPathExtractsPathFromURL(t *testing.T) {
	if got := endpointPath("http://192.168.51.1/api/gps/position/status"); got != "/api/gps/position/status" {
		t.Fatalf("got path %q", got)
	}
}
