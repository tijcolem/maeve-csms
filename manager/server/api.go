// SPDX-License-Identifier: Apache-2.0

package server

import (
	"github.com/rs/cors"
	"github.com/thoughtworks/maeve-csms/manager/adminui"
	"github.com/thoughtworks/maeve-csms/manager/api"
	"github.com/thoughtworks/maeve-csms/manager/config"
	"github.com/thoughtworks/maeve-csms/manager/handlers"
	"github.com/thoughtworks/maeve-csms/manager/handlers/ocpp16"
	"github.com/thoughtworks/maeve-csms/manager/handlers/ocpp201"
	"github.com/thoughtworks/maeve-csms/manager/ocpi"
	"github.com/thoughtworks/maeve-csms/manager/services"
	"github.com/thoughtworks/maeve-csms/manager/store"
	"github.com/thoughtworks/maeve-csms/manager/transport"
	"github.com/unrolled/secure"
	"k8s.io/utils/clock"
	"net/http"
	"os"
	"text/template"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/thoughtworks/maeve-csms/manager/templates"
)

func NewApiHandler(settings config.ApiSettings, engine store.Engine, ocpi ocpi.Api, csCertProvider services.ChargeStationCertificateProvider, emitter transport.Emitter) http.Handler {
	// Create CallMakers for both OCPP versions
	var ocpp16CallMaker handlers.CallMaker
	var ocpp201CallMaker handlers.CallMaker
	if emitter != nil {
		ocpp16CallMaker = ocpp16.NewCallMaker(emitter)
		ocpp201CallMaker = ocpp201.NewCallMaker(emitter)
	}

	apiServer, err := api.NewServer(engine, clock.RealClock{}, ocpi, ocpp16CallMaker, ocpp201CallMaker)
	if err != nil {
		panic(err)
	}

	var isDevelopment bool
	if os.Getenv("ENVIRONMENT") == "dev" {
		isDevelopment = true
	}
	secureMiddleware := secure.New(secure.Options{
		IsDevelopment:         isDevelopment,
		BrowserXssFilter:      true,
		ContentTypeNosniff:    true,
		FrameDeny:             true,
		ContentSecurityPolicy: "frame-ancestors: 'none'",
	})

	r := chi.NewRouter()

	logger := middleware.RequestLogger(logFormatter{})

	r.Use(middleware.Recoverer, secureMiddleware.Handler, cors.Default().Handler, api.ValidationMiddleware)
	r.Get("/health", health)
	r.Get("/transactions", transactions(engine))
	r.Handle("/metrics", promhttp.Handler())
	r.Get("/api/openapi.json", getApiSwaggerJson)
	r.With(logger).Mount("/api/v0", api.Handler(apiServer))
	r.With(logger).Mount("/adminui", adminui.NewServer(settings.Host, settings.WsPort, settings.WssPort, settings.OrgName, engine, csCertProvider))
	return r
}

func getApiSwaggerJson(w http.ResponseWriter, r *http.Request) {
	swagger, err := api.GetSwagger()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	json, err := swagger.MarshalJSON()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(json)
}

func health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"OK"}`))
}

func transactions(transactionStore store.Engine) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ts, err := transactionStore.Transactions(r.Context())
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		t := template.Must(template.New("").Parse(templates.Transactions))
		err = t.Execute(w, ts)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
