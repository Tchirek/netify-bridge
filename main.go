package main

import (
    "strings"
    "embed"
    "encoding/json"
    "flag"
    "log"
    "net"
    "net/http"
    "sync"
    "time"
)

//go:embed index.html
var static embed.FS

type Flow struct {
    LocalIP   string `json:"local_ip"`
    LocalMAC  string `json:"local_mac"`
    LocalPort int    `json:"local_port"`
    OtherIP   string `json:"other_ip"`
    OtherPort int    `json:"other_port"`
    AppName   string `json:"detected_application_name"`
    AppID     int    `json:"detected_application"`
    ProtoName string `json:"detected_protocol_name"`
    Host      string `json:"host_server_name"`
    SNIClient string `json:"client_sni"`
    LocalBytes int64 `json:"local_bytes"`
    OtherBytes int64 `json:"other_bytes"`
    FirstSeen int64  `json:"first_seen_at"`
    LastSeen  int64  `json:"last_seen_at"`
}

type SinkMsg struct {
    Flow      *Flow  `json:"flow"`
    Interface string `json:"interface"`
    Type      string `json:"type"`
}

type DeviceInfo struct {
    IP       string           `json:"ip"`
    MAC      string           `json:"mac"`
    Apps     map[string]int64 `json:"apps"`
    Bytes    int64            `json:"bytes"`
    Count    int64            `json:"count"`
    LastHost string           `json:"last_host"`
    LastApp  string           `json:"last_app"`
    LastSeen int64            `json:"last_seen"`
}

type AppStat struct {
    Name  string `json:"name"`
    Flows int64  `json:"flows"`
    Bytes int64  `json:"bytes"`
}

var (
    mu      sync.RWMutex
    devices = map[string]*DeviceInfo{}
    apps    = map[string]*AppStat{}
    recent  []*SinkMsg
    totalFlows int64
    totalBytes int64
)

func processMsg(m *SinkMsg) {
    f := m.Flow
    if f == nil {
        return
    }
    name := f.AppName
    if name == "" {
        name = "Unknown"
    }
    b := f.LocalBytes + f.OtherBytes
    mu.Lock()
    d := devices[f.LocalIP]
    if d == nil {
        d = &DeviceInfo{IP: f.LocalIP, Apps: map[string]int64{}}
        devices[f.LocalIP] = d
    }
    d.MAC = f.LocalMAC
    d.Apps[name]++
    d.Bytes += b
    d.Count++
    if f.Host != "" {
        d.LastHost = f.Host
    }
    if f.SNIClient != "" {
                d.LastHost = f.SNIClient
            }
    d.LastApp = name
    d.LastSeen = time.Now().Unix()

    a := apps[name]
    if a == nil {
        a = &AppStat{Name: name}
        apps[name] = a
    }
    a.Flows++
    a.Bytes += b
    totalFlows++
    totalBytes += b
    recent = append(recent, m)
    if len(recent) > 1000 {
        recent = append([]*SinkMsg{}, recent[950:]...)
    }
    mu.Unlock()
}

func connectSink(addr string) {
    for {
        log.Printf("connecting sink: %s", addr)
        var c net.Conn
        var err error
        target := addr
        if i := strings.Index(target, "://"); i > 0 {
            target = target[i+3:]
        }
        c, err = net.DialTimeout("tcp", target, 5*time.Second)
        if err != nil {
            log.Printf("dial error: %v", err)
            time.Sleep(5 * time.Second)
            continue
        }
        log.Printf("sink connected")
        dec := json.NewDecoder(c)
        for {
            var raw map[string]json.RawMessage
            if err := dec.Decode(&raw); err != nil {
                log.Printf("sink read error: %v", err)
                break
            }
            iface := ""
            typ := ""
            json.Unmarshal(raw["interface"], &iface)
            json.Unmarshal(raw["type"], &typ)
            if iface != "nfq-lan" || typ == "flow_purge" {
                continue
            }
            var m SinkMsg
            m.Interface = iface
            m.Type = typ
            if fr, ok := raw["flow"]; ok {
                f := Flow{}
                json.Unmarshal(fr, &f)
                var sslObj map[string]interface{}
                if err := json.Unmarshal(fr, &sslObj); err == nil {
                    if ssl, ok := sslObj["ssl"].(map[string]interface{}); ok {
                        if sni, _ := ssl["client_sni"].(string); sni != "" {
                            f.SNIClient = sni
                        }
                    }
                }
                m.Flow = &f
            }
            processMsg(&m)
        }
        c.Close()
        time.Sleep(3 * time.Second)
    }
}

func apiStats(w http.ResponseWriter, r *http.Request) {
    mu.RLock()
    defer mu.RUnlock()
    out := map[string]interface{}{
        "devices_count": len(devices),
        "total_flows":   totalFlows,
        "total_bytes":   totalBytes,
        "apps":          apps,
    }
    writeJSON(w, out)
}

func apiDevices(w http.ResponseWriter, r *http.Request) {
    mu.RLock()
    defer mu.RUnlock()
    var out []*DeviceInfo
    for _, d := range devices {
        out = append(out, d)
    }
    writeJSON(w, out)
}

func apiFlows(w http.ResponseWriter, r *http.Request) {
    mu.RLock()
    defer mu.RUnlock()
    out := recent
    if len(out) > 100 {
        out = out[len(out)-100:]
    }
    writeJSON(w, out)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(v)
}

func main() {
    addr := flag.String("sink", "tcp://192.168.10.1:1750", "netify sink address (tcp://host:port or unix://path)")
    listen := flag.String("listen", ":8080", "HTTP listen address")
    flag.Parse()
    go connectSink(*addr)
    http.HandleFunc("/api/stats", apiStats)
    http.HandleFunc("/api/devices", apiDevices)
    http.HandleFunc("/api/flows", apiFlows)
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        data, _ := static.ReadFile("index.html")
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Write(data)
    })
    log.Printf("listening on %s", *listen)
    log.Fatal(http.ListenAndServe(*listen, nil))
}
