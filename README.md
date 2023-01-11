# Mikrotik login/logout metric exporter

This tool:

* Reads log stream emitted by Mikrotik routers
* Rarses VPN login and logout messages
* Takes VPN client's IP address and looks it up in a local
  [GeoLite2 ASN](
  https://dev.maxmind.com/geoip/docs/databases/asn?lang=en)
  database.
* Exports login/logout Prometheus metrics broken down by ASN.

Log messages are expected to be shipped to the TCP syslog endpoint.

## License

Licensed under MIT license.
