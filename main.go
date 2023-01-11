package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/oschwald/geoip2-golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/mcuadros/go-syslog.v2"
)

var labelNames = []string{"device", "protocol", "asn"}

var logoutDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:                         "mikrotik_conn_session_duration_seconds",
	Buckets:                      []float64{1, 10, 60, 120, 300, 600, 900, 1800, 3600, 7200, 14400, 28800, 57600, 86400, 129600, 172800},
	NativeHistogramBucketFactor:  1.1,
	NativeHistogramZeroThreshold: 1,
}, labelNames)

var loginCount = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "mikrotik_conn_logins",
}, labelNames)

func main() {
	listenS := flag.String("syslog-listen", "0.0.0.0:2514", "syslog TCP listen ip:port")
	listenH := flag.String("http-listen", "0.0.0.0:8122", "http listen ip:port")
	translate := flag.String("device-names", "", "comma separated list of ipaddr/host pairs used to look up device name (e.g. '10.11.12.13/router1')")
	geoipFile := flag.String("geoip-file", "GeoLite2-ASN.mmdb", "path to the geoip ASN database")
	flag.Parse()

	devices := make(map[string]string)
	for _, kv := range strings.Split(*translate, ",") {
		if kv == "" {
			continue
		}
		f := strings.Split(kv, "/")
		devices[f[0]] = f[1]
	}

	db, err := geoip2.Open(*geoipFile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	prometheus.MustRegister(logoutDuration)
	prometheus.MustRegister(loginCount)
	http.Handle("/metrics", promhttp.Handler())
	go func() { log.Fatal(http.ListenAndServe(*listenH, nil)) }()

	channel := make(syslog.LogPartsChannel)
	handler := syslog.NewChannelHandler(channel)

	server := syslog.NewServer()
	server.SetFormat(syslog.RFC6587)
	server.SetHandler(handler)
	if err := server.ListenTCP(*listenS); err != nil {
		log.Fatal(err)
	}
	if err := server.Boot(); err != nil {
		log.Fatal(err)
	}

	lookupASN := func(ipaddr string) (uint, error) {
		ip := net.ParseIP(ipaddr)
		if ip == nil {
			return 0, fmt.Errorf("could not parse %q as IP", ipaddr)
		}
		record, err := db.ASN(ip)
		if err != nil {
			return 0, fmt.Errorf("could not look up %s: %s", ip, err)
		}
		return record.AutonomousSystemNumber, nil
	}

	loginRe := regexp.MustCompile(`\S+ logged in, \S+ from (\S+)`)
	logoutRe := regexp.MustCompile(`\S+ logged out, (\d+) \d+ \d+ \d+ \d+ from (\S+)`)
	go func(channel syslog.LogPartsChannel) {
		for logParts := range channel {
			protocol := strings.Split(logParts["app_name"].(string), ",")[0]
			if protocol != "ovpn" && protocol != "sstp" {
				continue
			}

			device := logParts["hostname"].(string)
			if name, ok := devices[device]; ok {
				device = name
			}

			labels := prometheus.Labels{"device": device, "protocol": protocol}

			if m := loginRe.FindStringSubmatch(logParts["message"].(string)); m != nil {
				asn, err := lookupASN(m[1])
				if err != nil {
					log.Printf("ERROR: %s", err)
					continue
				}
				labels["asn"] = fmt.Sprintf("%d", asn)
				loginCount.With(labels).Inc()
			}
			if m := logoutRe.FindStringSubmatch(logParts["message"].(string)); m != nil {
				asn, err := lookupASN(m[2])
				if err != nil {
					log.Printf("ERROR: %s", err)
					continue
				}
				labels["asn"] = fmt.Sprintf("%d", asn)
				val, err := strconv.ParseFloat(m[1], 64)
				if err != nil {
					log.Printf("ERROR: could not parse %q: %s", m[1], err)
					continue
				}
				logoutDuration.With(labels).Observe(val)
			}
		}
	}(channel)

	server.Wait()
}
