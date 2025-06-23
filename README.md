# leontp-exporter

This is a [Prometheus](https://prometheus.io/) exporter for collecting
statistics fom
[LeoNTP](https://www.leobodnar.com/shop/index.php?main_page=product_info&products_id=272&srsltid=AfmBOoogmcrUP1COntYSWalTsfuGFRj5wH_s6_3SSJUOHFT51YI6Fugn)
NTP appliances.  It's largely based on
[sean-foley/leo-ntp-monitor](https://github.com/sean-foley/leo-ntp-monitor),
except it (a) talks to Prometheus instead of InfluxDB and (b) it's
written in Go.

To build, just checkout and use `go build`.  It listens on port 9124
by default, but this can be changed with the `--listen` flag.  A
minimal systemd unit file is included.

## Collecting Data with Prometheus

A single `leontp-exporter` instance can collect data from multiple
LeoNTP devices.  It uses the `target` HTTP query param to identify
which LeoNTP device to talk to, similar to the way that the Prometheus
[SNMP Exporter](https://github.com/prometheus/snmp_exporter) works.
To collect stats from a pair of LeoNTP devices named
`leontp1.example.com` and `leontp2.example.com` via an exporter
running on the local machine, you can use this `prometheus.yml`
snippet:

```
  - job_name: "leontp"
    scrape_timeout: 10s
    scrape_interval: 15s
    static_configs:
      - targets:
        - leontp1.example.com
        - leontp2.example.com
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: localhost:9124
```

## Metrics

This exporter only exports 4 metrics right now:

```
# HELP leontp_lock_time_seconds Number of seconds that this device has been locked to GPS
# TYPE leontp_lock_time_seconds gauge
leontp_lock_time_seconds 435022
# HELP leontp_ntp_request_count Number of NTP requests since the device's last reboot
# TYPE leontp_ntp_request_count gauge
leontp_ntp_request_count 2.355704e+07
# HELP leontp_satellites_count Current number of visible satellites
# TYPE leontp_satellites_count gauge
leontp_satellites_count 16
# HELP leontp_uptime_seconds Number of seconds this device has been running since its last reboot
# TYPE leontp_uptime_seconds gauge
leontp_uptime_seconds 435024
```

## Status

It compiles and seems to work for me.  I'm using it to collect stats,
but it's not very polished code.  It was thrown together in a couple
hours on a Sunday afternoon.
