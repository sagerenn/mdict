package observability

import "expvar"

var (
	RequestsTotal = expvar.NewInt("requests_total")
	Responses2xx  = expvar.NewInt("responses_2xx")
	Responses4xx  = expvar.NewInt("responses_4xx")
	Responses5xx  = expvar.NewInt("responses_5xx")
)

func recordStatus(code int) {
	RequestsTotal.Add(1)
	switch {
	case code >= 200 && code < 300:
		Responses2xx.Add(1)
	case code >= 400 && code < 500:
		Responses4xx.Add(1)
	case code >= 500:
		Responses5xx.Add(1)
	}
}
