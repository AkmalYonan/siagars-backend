package service

import (
	"math"
	"net/url"
	"siagars/backend/internal/domain"
	"testing"
)

// The coarse database box must never lose a hospital that the exact Haversine radius keeps.
func TestRadiusBBoxContainsCircle(t *testing.T) {
	for _, c := range []struct{ lat, lng, km float64 }{{-6.2, 106.8, 5}, {-6.2, 106.8, 50}, {3.6, 98.7, 20}, {-8.6, 115.2, 10}} {
		box := RadiusBBox(c.lat, c.lng, c.km)
		for deg := 0.0; deg < 360; deg += 5 {
			// Walk the circle boundary with small steps until Haversine says we are at radius km.
			lat, lng := c.lat, c.lng
			for domain.Distance(c.lat, c.lng, lat, lng) < c.km*0.999 {
				lat += 0.0005 * math.Cos(deg*math.Pi/180)
				lng += 0.0005 * math.Sin(deg*math.Pi/180)
			}
			h := domain.Hospital{Latitude: &lat, Longitude: &lng}
			if !box.Contains(h) {
				t.Fatalf("%+v: titik %.4f,%.4f (arah %v°) di luar box %+v", c, lat, lng, deg, box)
			}
		}
	}
}

func TestParseRecommendDefaults(t *testing.T) {
	v, _ := url.ParseQuery("services=nicu,icu&lat=-6.2&lng=106.8")
	q, e := ParseRecommend(v)
	if e != nil || q.RadiusKM != 20 || q.Limit != 10 || q.Dataset != "real" || q.BPJSRequired || q.Area == nil || len(q.Services) != 2 {
		t.Fatalf("%+v %v", q, e)
	}
	v, _ = url.ParseQuery("services=nicu&city_code=3275&bpjs=required&limit=20&dataset=all")
	if q, e = ParseRecommend(v); e != nil || !q.BPJSRequired || q.Area != nil || q.Limit != 20 || q.City != "3275" {
		t.Fatalf("%+v %v", q, e)
	}
}

func TestParseMarkersBounds(t *testing.T) {
	for raw, ok := range map[string]bool{"bbox=106,-7,108,-5": true, "bbox=106,-7,108.0001,-5": false, "bbox=106,-7,106,-5": false, "bbox=106,-7,107,-6&dataset=all&bpjs=false&services=icu": true} {
		v, _ := url.ParseQuery(raw)
		if _, e := ParseMarkers(v); (e == nil) != ok {
			t.Errorf("%s: %v", raw, e)
		}
	}
}
