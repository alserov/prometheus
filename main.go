package main

import (
	"encoding/json"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Device struct {
	ID       int    `json:"id"`
	Mac      string `json:"mac"`
	Firmware string `json:"firmware"`
}

var devices []*Device
var version string

func init() {
	version = "2.10.5"
	devices = []*Device{
		{
			ID:       1,
			Mac:      "5F-4G",
			Firmware: "2.2.0",
		},
		{
			ID:       2,
			Mac:      "5E-4G",
			Firmware: "2.2.1",
		},
	}
}

type metrics struct {
	devices  prometheus.Gauge
	info     *prometheus.GaugeVec
	upgrades *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func NewMetrics(r prometheus.Registerer) *metrics {
	m := &metrics{
		devices: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "myapp",
			Name:      "devices",
			Help:      "all devices",
		}),
		info: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "myapp",
			Name:      "info",
			Help:      "app env info",
		}, []string{"version"}),
		upgrades: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "myapp",
			Name:      "device_upgrade_total",
			Help:      "Number of upgraded devices",
		}, []string{"type"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "myapp",
			Name:      "request_duration_seconds",
			Help:      "request duration",
			Buckets:   []float64{0.1, 0.15, 0.2, 0.3},
		}, []string{"status", "method"}),
	}
	r.MustRegister(m.devices, m.info, m.upgrades, m.duration)
	return m
}

func main() {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)
	//reg.MustRegister(collectors.NewGoCollector())

	m.devices.Set(float64(len(devices)))
	m.info.With(prometheus.Labels{"version": version}).Set(1)

	dMux := http.NewServeMux()
	mdh := manageDevicesHandler{
		metrics: m,
	}
	dMux.HandleFunc("/metrics/devices", mdh.ServeHTTP)
	dMux.HandleFunc("/metrics/devices/", mdh.ServeHTTP)

	pMux := http.NewServeMux()
	prmHandler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	pMux.Handle("/metrics", prmHandler)

	go func() {
		log.Fatalln(http.ListenAndServe(":3001", dMux))
	}()

	go func() {
		log.Fatalln(http.ListenAndServe(":8001", pMux))
	}()

	select {}
}

func getDevices(w http.ResponseWriter, r *http.Request, m *metrics) {
	now := time.Now()

	b, err := json.Marshal(devices)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	time.Sleep(200)

	m.duration.With(prometheus.Labels{"method": "GET", "status": "200"}).Observe(time.Since(now).Seconds())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func upgradeDevice(w http.ResponseWriter, r *http.Request, m *metrics) {
	path := strings.TrimPrefix(r.URL.Path, "/metrics/devices/")

	id, err := strconv.Atoi(path)
	if err != nil || id < 1 {
		http.NotFound(w, r)
	}

	var device Device
	if err = json.NewDecoder(r.Body).Decode(&device); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for i := range devices {
		if devices[i].ID == id {
			devices[i].Firmware = device.Firmware
		}
	}

	m.upgrades.With(prometheus.Labels{"type": "router"}).Inc()

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("upgrading"))
}

type manageDevicesHandler struct {
	metrics *metrics
}

func (mdh manageDevicesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getDevices(w, r, mdh.metrics)
	case http.MethodPut:
		upgradeDevice(w, r, mdh.metrics)
	default:
		w.Header().Set("Allow", "PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
