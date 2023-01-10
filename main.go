package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/oschwald/geoip2-golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/mcuadros/go-syslog.v2"
)

var labelNames = []string{"device", "protocol", "asn"}

var logoutDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name: "mikrotik_conn_session_duration_seconds",
}, labelNames)

var loginCount = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "mikrotik_conn_logins",
}, labelNames)

func main() {
	listenS := flag.String("syslog-listen", "0.0.0.0:2514", "syslog TCP listen ip:port")
	listenH := flag.String("http-listen", "0.0.0.0:8122", "http listen ip:port")
	flag.Parse()

	db, err := geoip2.Open("GeoLite2-ASN.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ip := net.ParseIP("81.2.69.142")
	record, err := db.ASN(ip)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%+v", record.AutonomousSystemNumber)

	prometheus.MustRegister(logoutDuration)
	prometheus.MustRegister(loginCount)
	http.Handle("/metrics", promhttp.Handler())
	go func() { log.Fatal(http.ListenAndServe(*listenH, nil)) }()

	channel := make(syslog.LogPartsChannel)
	handler := syslog.NewChannelHandler(channel)

	server := syslog.NewServer()
	server.SetFormat(syslog.RFC5424)
	server.SetHandler(handler)
	server.ListenTCP(*listenS)
	server.Boot()

	go func(channel syslog.LogPartsChannel) {
		for logParts := range channel {
			fmt.Println(logParts)
		}
	}(channel)

	server.Wait()
}
