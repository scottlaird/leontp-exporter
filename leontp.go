package main

import (
	"fmt"
	"flag"
	"time"
	"net"
	"encoding/binary"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

)

var (
	listenAddr = flag.String("listen", ":9124", "HTTP port and optionally address to listen to for requests")
)

type LeoNTPStatus struct {
	RefTS0, RefTS1,	Uptime, NTPRequests, LockTime, Firmware uint32
	Flags uint8
	Satellites uint8
	SerialNumber uint16
	TimeStamp time.Time
}


type LeoNTPCollector struct {
	hostname string
	Uptime *prometheus.Desc
	NTPRequests *prometheus.Desc
	LockTime *prometheus.Desc
	Satellites *prometheus.Desc
}

func NewLeoNTPCollector(hostname string) *LeoNTPCollector {
	return &LeoNTPCollector{
		hostname: hostname,
		Uptime: prometheus.NewDesc("leontp_uptime_seconds", "Number of seconds this device has been running since its last reboot", nil, nil),
		NTPRequests: prometheus.NewDesc("leontp_ntp_request_count", "Number of NTP requests since the device's last reboot", nil, nil),
		LockTime: prometheus.NewDesc("leontp_lock_time_seconds", "Number of seconds that this device has been locked to GPS", nil, nil),
		Satellites: prometheus.NewDesc("leontp_satellites_count", "Current number of visible satellites", nil, nil),
	}
}

func (c *LeoNTPCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Uptime
	ch <- c.NTPRequests
	ch <- c.LockTime
	ch <- c.Satellites
}

func (c *LeoNTPCollector) Collect(ch chan<- prometheus.Metric) {
	hostPort := fmt.Sprintf("%s:123", c.hostname)
	status, err := GetNTPMetrics(hostPort, time.Second)

	if err != nil {
		slog.Error("Unable to get metrics", "error", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(c.Uptime, prometheus.GaugeValue, float64(status.Uptime))
	ch <- prometheus.MustNewConstMetric(c.NTPRequests, prometheus.GaugeValue, float64(status.NTPRequests))
	ch <- prometheus.MustNewConstMetric(c.LockTime, prometheus.GaugeValue, float64(status.LockTime))
	ch <- prometheus.MustNewConstMetric(c.Satellites, prometheus.GaugeValue, float64(status.Satellites))
}




// Attempt at a quick port of
// https://github.com/sean-foley/leo-ntp-monitor/blob/main/leo-ntp-monitor.py,
// but for Prometheus instead of Influxdb.
func GetNTPMetrics(hostPort string, timeout time.Duration) (LeoNTPStatus, error) {
	sendBuffer := []byte{4<<3+7, 0, 0x10, 1, 0, 0, 0, 0, 0}
	result := LeoNTPStatus{}

	addr, err := net.ResolveUDPAddr("udp", hostPort)
	if err != nil {
		return result, fmt.Errorf("Unable to resolve hostname: %v", err)
	}
	
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return result, fmt.Errorf("Unable to connect: %v", err)
	}

	conn.SetDeadline(time.Now().Add(timeout))

	_, err = conn.Write(sendBuffer)
	if err != nil {
		return result, fmt.Errorf("Unable to send: %v", err)
	}

	b := make([]byte, 1024)  // Grossly oversized, but whatever.
	_, err = conn.Read(b)
	if err != nil {
		return result, fmt.Errorf("Unable to read: %v", err)
	}

	result.RefTS0 = binary.LittleEndian.Uint32(b[16:])
	result.RefTS1 = binary.LittleEndian.Uint32(b[20:])
	result.Uptime = binary.LittleEndian.Uint32(b[24:])
	result.NTPRequests = binary.LittleEndian.Uint32(b[28:])
	result.LockTime = binary.LittleEndian.Uint32(b[36:])
	result.Flags = b[40]
	result.Satellites = b[41]
	result.SerialNumber = binary.LittleEndian.Uint16(b[42:])
	result.Firmware = binary.LittleEndian.Uint32(b[44:])

	result.TimeStamp =  ParseNTPTime(result.RefTS1, result.RefTS0)

	return result, nil	
}


// Convert uint32 seconds-since-1900 and fractional seconds into a time.Time
func ParseNTPTime(ts1, ts0 uint32) time.Time {
	secs := int64(ts1 - 2208988800) // Convert from Jan 1, 1900 to Jan 1, 1970.
	nsecs := int64(float64(ts0) / (1<<32) * 1e9) // Convert from a fraction of a second to nanoseconds. 
	return time.Unix(secs, nsecs)
}

type handler struct {
	leontpHost string
}

func newHandler() *handler {
	return &handler{}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query()["target"]
	if len(target) != 1 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Must specify ?target=<leontp address>"))
		return
	}

	c := NewLeoNTPCollector(target[0])
	reg := prometheus.NewRegistry()
	reg.MustRegister(c)

	leoNTPHandler := promhttp.HandlerFor(
		reg,
		promhttp.HandlerOpts{
			ErrorHandling:       promhttp.ContinueOnError,
		},
	)
	leoNTPHandler.ServeHTTP(w, r)
}

func main() {
	flag.Parse()
	http.Handle("/metrics", newHandler())
	http.ListenAndServe(*listenAddr, nil)
}
