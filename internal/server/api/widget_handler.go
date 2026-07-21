package api

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"hawkeye/internal/server/storage"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"
)

var proxyClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

// --- Widget CRUD ---

func listWidgets(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		widgets, err := db.ListWidgets()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if widgets == nil {
			widgets = []storage.Widget{}
		}
		c.JSON(http.StatusOK, widgets)
	}
}

func createWidget(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var w storage.Widget
		if err := c.ShouldBindJSON(&w); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if w.Name == "" || w.Type == "" || (w.URL == "" && w.Type != "hawkeye") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name, type, url are required"})
			return
		}
		w.Enabled = true
		if err := db.CreateWidget(&w); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, w)
	}
}

func updateWidget(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		existing, err := db.GetWidget(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if existing == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "widget not found"})
			return
		}

		var update storage.Widget
		if err := c.ShouldBindJSON(&update); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Merge: only overwrite fields that were actually sent
		if update.Name != "" {
			existing.Name = update.Name
		}
		if update.Type != "" {
			existing.Type = update.Type
		}
		if update.URL != "" {
			existing.URL = update.URL
		}
		if update.APIToken != "" {
			existing.APIToken = update.APIToken
		}
		if update.Node != "" {
			existing.Node = update.Node
		}
		if update.Config != "" {
			existing.Config = update.Config
		}
		if update.Description != "" {
			existing.Description = update.Description
		}
		if update.WidgetGroup != "" {
			existing.WidgetGroup = update.WidgetGroup
		}
		if update.Enabled != existing.Enabled {
			existing.Enabled = update.Enabled
		}
		if update.SortOrder != 0 {
			existing.SortOrder = update.SortOrder
		}

		if err := db.UpdateWidget(existing); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, existing)
	}
}

func deleteWidget(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		if err := db.DeleteWidget(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	}
}

// --- Widget Data Proxy ---

// pingHost measures TCP connect latency to a URL's host:port. Returns ms or -1 if unreachable.
func pingHost(rawURL string) int64 {
	u, err := url.Parse(rawURL)
	if err != nil {
		return -1
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "https" {
			host += ":443"
		} else {
			host += ":80"
		}
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", host, 3*time.Second)
	if err != nil {
		return -1
	}
	conn.Close()
	return time.Since(start).Milliseconds()
}

func getWidgetData(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		w, err := db.GetWidget(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if w == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "widget not found"})
			return
		}

		// Ping the widget's host
		pingMs := int64(-1)
		if w.URL != "" && w.Type != "hawkeye" {
			pingMs = pingHost(w.URL)
		}

		var data interface{}
		switch w.Type {
		case "proxmox":
			data, err = fetchProxmoxData(w)
		case "pbs":
			data, err = fetchPBSData(w)
		case "unraid":
			data, err = fetchUnraidData(w)
		case "portainer":
			data, err = fetchPortainerData(w)
		case "adguard":
			data, err = fetchAdGuardData(w)
		case "jellyfin":
			data, err = fetchJellyfinData(w)
		case "moviepilot":
			data, err = fetchMoviePilotData(w)
		case "qbittorrent":
			data, err = fetchQBittorrentData(w)
		case "hawkeye":
			data, err = fetchHawkeyeData(db)
		case "lucky":
			data, err = fetchLuckyData(w)
		case "transmission":
			data, err = fetchTransmissionData(w)
		case "homeassistant":
			data, err = fetchHomeAssistantData(w)
		case "openwrt":
			data, err = fetchOpenWrtData(w)
		case "ikuai":
			data, err = fetchIkuaiData(w)
		case "openclash":
			data, err = fetchOpenClashData(w)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported widget type: " + w.Type})
			return
		}

		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "ping_ms": pingMs})
			return
		}

		// Wrap data with ping info
		dataBytes, _ := json.Marshal(data)
		var result map[string]interface{}
		json.Unmarshal(dataBytes, &result)
		if result == nil {
			result = make(map[string]interface{})
		}
		if w.Type != "hawkeye" {
			result["ping_ms"] = pingMs
		}
		c.JSON(http.StatusOK, result)
	}
}

// --- Proxmox VE ---

type ProxmoxData struct {
	Node    string          `json:"node"`
	Status  string          `json:"status"`
	CPU     float64         `json:"cpu"`
	MaxCPU  int             `json:"maxcpu"`
	Mem     int64           `json:"mem"`
	MaxMem  int64           `json:"maxmem"`
	Uptime  int64           `json:"uptime"`
	VMs     []ProxmoxVM     `json:"vms"`
	CTs     []ProxmoxVM     `json:"cts"`
}

type ProxmoxVM struct {
	VMID   int     `json:"vmid"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	Mem    int64   `json:"mem"`
	MaxMem int64   `json:"maxmem"`
}

func fetchProxmoxData(w *storage.Widget) (*ProxmoxData, error) {
	node := w.Node
	if node == "" {
		node = "pve"
	}

	result := &ProxmoxData{Node: node}

	type apiResult struct {
		name string
		data map[string]interface{}
		err  error
	}

	ch := make(chan apiResult, 3)

	// 1. Node status
	go func() {
		data, err := proxmoxAPIGet(w.URL, w.APIToken, fmt.Sprintf("/api2/json/nodes/%s/status", node))
		ch <- apiResult{"status", data, err}
	}()

	// 2. VMs (QEMU)
	go func() {
		data, err := proxmoxAPIGet(w.URL, w.APIToken, fmt.Sprintf("/api2/json/nodes/%s/qemu", node))
		ch <- apiResult{"qemu", data, err}
	}()

	// 3. CTs (LXC)
	go func() {
		data, err := proxmoxAPIGet(w.URL, w.APIToken, fmt.Sprintf("/api2/json/nodes/%s/lxc", node))
		ch <- apiResult{"lxc", data, err}
	}()

	// Collect results
	for i := 0; i < 3; i++ {
		r := <-ch
		switch r.name {
		case "status":
			if r.err != nil {
				return nil, fmt.Errorf("get node status: %w", r.err)
			}
			if data, ok := r.data["data"].(map[string]interface{}); ok {
				if cpu, ok := data["cpu"].(float64); ok {
					result.CPU = cpu
				}
				if maxcpu, ok := data["cpuinfo"].(map[string]interface{}); ok {
					if cores, ok := maxcpu["cores"].(float64); ok {
						if sockets, ok := maxcpu["sockets"].(float64); ok {
							result.MaxCPU = int(cores * sockets)
						} else {
							result.MaxCPU = int(cores)
						}
					}
				}
				if mem, ok := data["memory"].(map[string]interface{}); ok {
					if used, ok := mem["used"].(float64); ok {
						result.Mem = int64(used)
					}
					if total, ok := mem["total"].(float64); ok {
						result.MaxMem = int64(total)
					}
				}
				if uptime, ok := data["uptime"].(float64); ok {
					result.Uptime = int64(uptime)
				}
			}
			result.Status = "online"

		case "qemu":
			if r.err == nil {
				if dataArr, ok := r.data["data"].([]interface{}); ok {
					for _, item := range dataArr {
						if vm, ok := item.(map[string]interface{}); ok {
							if tmpl, ok := vm["template"].(float64); ok && tmpl == 1 {
								continue
							}
							p := parseProxmoxVM(vm)
							if p.Status != "running" {
								continue
							}
							result.VMs = append(result.VMs, p)
						}
					}
					sort.Slice(result.VMs, func(i, j int) bool { return result.VMs[i].VMID < result.VMs[j].VMID })
				}
			}

		case "lxc":
			if r.err == nil {
				if dataArr, ok := r.data["data"].([]interface{}); ok {
					for _, item := range dataArr {
						if ct, ok := item.(map[string]interface{}); ok {
							if tmpl, ok := ct["template"].(float64); ok && tmpl == 1 {
								continue
							}
							p := parseProxmoxVM(ct)
							if p.Status != "running" {
								continue
							}
							result.CTs = append(result.CTs, p)
						}
					}
					sort.Slice(result.CTs, func(i, j int) bool { return result.CTs[i].VMID < result.CTs[j].VMID })
				}
			}
		}
	}

	return result, nil
}

func parseProxmoxVM(vm map[string]interface{}) ProxmoxVM {
	p := ProxmoxVM{}
	if vmid, ok := vm["vmid"].(float64); ok {
		p.VMID = int(vmid)
	}
	if name, ok := vm["name"].(string); ok {
		p.Name = name
	}
	if status, ok := vm["status"].(string); ok {
		p.Status = status
	}
	if cpu, ok := vm["cpu"].(float64); ok {
		p.CPU = cpu
	}
	if mem, ok := vm["mem"].(float64); ok {
		p.Mem = int64(mem)
	}
	if maxmem, ok := vm["maxmem"].(float64); ok {
		p.MaxMem = int64(maxmem)
	}
	return p
}

func proxmoxAPIGet(baseURL, token, path string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "PVEAPIToken="+token)

	resp, err := proxyClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// --- Proxmox Backup Server ---

type PBSData struct {
	Datastore string  `json:"datastore"`
	Total     int64   `json:"total"`
	Used      int64   `json:"used"`
	Available int64   `json:"available"`
	UsedPct   float64 `json:"used_percent"`
	Snapshots int     `json:"snapshots"`
	CPU       float64 `json:"cpu"`
	MaxCPU    int     `json:"maxcpu"`
	Mem       int64   `json:"mem"`
	MaxMem    int64   `json:"maxmem"`
	Uptime    int64   `json:"uptime"`
}

func fetchPBSData(w *storage.Widget) (*PBSData, error) {
	datastore := w.Node // reuse node field for datastore name
	if datastore == "" {
		datastore = "PBS"
	}

	result := &PBSData{Datastore: datastore}

	// Get node status (CPU/Memory/Uptime)
	nodeData, err := pbsAPIGet(w.URL, w.APIToken, "/api2/json/nodes/localhost/status")
	if err == nil {
		if data, ok := nodeData["data"].(map[string]interface{}); ok {
			if cpu, ok := data["cpu"].(float64); ok {
				result.CPU = cpu
			}
			if cpuinfo, ok := data["cpuinfo"].(map[string]interface{}); ok {
				if cpus, ok := cpuinfo["cpus"].(float64); ok {
					result.MaxCPU = int(cpus)
				}
			}
			if mem, ok := data["memory"].(map[string]interface{}); ok {
				if used, ok := mem["used"].(float64); ok {
					result.Mem = int64(used)
				}
				if total, ok := mem["total"].(float64); ok {
					result.MaxMem = int64(total)
				}
			}
			if uptime, ok := data["uptime"].(float64); ok {
				result.Uptime = int64(uptime)
			}
		}
	}

	// Get datastore status
	dsData, err := pbsAPIGet(w.URL, w.APIToken, "/api2/json/status/datastore-usage")
	if err != nil {
		return nil, fmt.Errorf("get datastore usage: %w", err)
	}

	if dataArr, ok := dsData["data"].([]interface{}); ok {
		for _, item := range dataArr {
			if ds, ok := item.(map[string]interface{}); ok {
				name, _ := ds["store"].(string)
				if name == datastore || datastore == "" {
					if total, ok := ds["total"].(float64); ok {
						result.Total = int64(total)
					}
					if used, ok := ds["used"].(float64); ok {
						result.Used = int64(used)
					}
					if avail, ok := ds["avail"].(float64); ok {
						result.Available = int64(avail)
					}
					if result.Total > 0 {
						result.UsedPct = float64(result.Used) / float64(result.Total) * 100
					}
					result.Datastore = name
					break
				}
			}
		}
	}

	// Get snapshot count
	snapData, err := pbsAPIGet(w.URL, w.APIToken, fmt.Sprintf("/api2/json/admin/datastore/%s/snapshots", datastore))
	if err == nil {
		if dataArr, ok := snapData["data"].([]interface{}); ok {
			result.Snapshots = len(dataArr)
		}
	}

	return result, nil
}

func pbsAPIGet(baseURL, token, path string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "PBSAPIToken="+token)

	resp, err := proxyClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// --- Unraid ---

type UnraidData struct {
	CPU         float64      `json:"cpu"`
	MemPct      float64      `json:"mem_percent"`
	MemActive   int64        `json:"mem_active"`
	MemAvail    int64        `json:"mem_available"`
	ArrayState  string       `json:"array_state"`
	ArrayTotal  int64        `json:"array_total"`
	ArrayUsed   int64        `json:"array_used"`
	ArrayFree   int64        `json:"array_free"`
	Pools       []UnraidPool `json:"pools"`
	NotifCount  int          `json:"notif_count"`
}

type UnraidPool struct {
	Name    string  `json:"name"`
	FsType  string  `json:"fs_type"`
	Total   int64   `json:"total"`
	Used    int64   `json:"used"`
	Free    int64   `json:"free"`
	UsedPct float64 `json:"used_percent"`
}

func fetchUnraidData(w *storage.Widget) (*UnraidData, error) {
	query := `{"query":"{ array { state capacity { kilobytes { free total used } } caches { name fsType fsSize fsFree fsUsed } } metrics { memory { active available percentTotal } cpu { percentTotal } } notifications { overview { unread { total } } } }"}`

	req, err := http.NewRequest("POST", w.URL+"/graphql", strings.NewReader(query))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", w.APIToken)

	resp, err := proxyClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Unraid API returned %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	var gqlResp struct {
		Data struct {
			Array struct {
				State    string `json:"state"`
				Capacity struct {
					Kilobytes struct {
						Free  string `json:"free"`
						Total string `json:"total"`
						Used  string `json:"used"`
					} `json:"kilobytes"`
				} `json:"capacity"`
				Caches []struct {
					Name   string   `json:"name"`
					FsType *string  `json:"fsType"`
					FsSize *float64 `json:"fsSize"`
					FsFree *float64 `json:"fsFree"`
					FsUsed *float64 `json:"fsUsed"`
				} `json:"caches"`
			} `json:"array"`
			Metrics struct {
				Memory struct {
					Active       int64   `json:"active"`
					Available    int64   `json:"available"`
					PercentTotal float64 `json:"percentTotal"`
				} `json:"memory"`
				CPU struct {
					PercentTotal float64 `json:"percentTotal"`
				} `json:"cpu"`
			} `json:"metrics"`
			Notifications struct {
				Overview struct {
					Unread struct {
						Total int `json:"total"`
					} `json:"unread"`
				} `json:"overview"`
			} `json:"notifications"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	d := gqlResp.Data
	result := &UnraidData{
		CPU:        d.Metrics.CPU.PercentTotal,
		MemPct:     d.Metrics.Memory.PercentTotal,
		MemActive:  d.Metrics.Memory.Active,
		MemAvail:   d.Metrics.Memory.Available,
		ArrayState: d.Array.State,
		NotifCount: d.Notifications.Overview.Unread.Total,
	}

	// Parse array capacity (kilobytes are strings in the API)
	if free, err := strconv.ParseInt(d.Array.Capacity.Kilobytes.Free, 10, 64); err == nil {
		result.ArrayFree = free * 1000
	}
	if total, err := strconv.ParseInt(d.Array.Capacity.Kilobytes.Total, 10, 64); err == nil {
		result.ArrayTotal = total * 1000
	}
	if used, err := strconv.ParseInt(d.Array.Capacity.Kilobytes.Used, 10, 64); err == nil {
		result.ArrayUsed = used * 1000
	}

	// Parse cache pools
	for _, cache := range d.Array.Caches {
		if cache.FsType == nil || *cache.FsType == "" {
			continue // skip pools without filesystem
		}
		pool := UnraidPool{
			Name:   cache.Name,
			FsType: *cache.FsType,
		}
		if cache.FsSize != nil {
			pool.Total = int64(*cache.FsSize) * 1000
		}
		if cache.FsUsed != nil {
			pool.Used = int64(*cache.FsUsed) * 1000
		}
		if cache.FsFree != nil {
			pool.Free = int64(*cache.FsFree) * 1000
		}
		if pool.Total > 0 {
			pool.UsedPct = float64(pool.Used) / float64(pool.Total) * 100
		}
		result.Pools = append(result.Pools, pool)
	}

	return result, nil
}

// --- Portainer ---

type PortainerData struct {
	Running int `json:"running"`
	Stopped int `json:"stopped"`
	Total   int `json:"total"`
}

func fetchPortainerData(w *storage.Widget) (*PortainerData, error) {
	// Get endpoints first
	endpointsData, err := portainerAPIGet(w.URL, w.APIToken, "/api/endpoints")
	if err != nil {
		return nil, fmt.Errorf("get endpoints: %w", err)
	}

	var endpoints []struct {
		ID int `json:"Id"`
	}
	endpointsBytes, _ := json.Marshal(endpointsData)
	json.Unmarshal(endpointsBytes, &endpoints)

	result := &PortainerData{}

	// Get containers from first endpoint (or use node field as endpoint ID)
	endpointID := "3"
	if w.Node != "" {
		endpointID = w.Node
	} else if len(endpoints) > 0 {
		endpointID = fmt.Sprintf("%d", endpoints[0].ID)
	}

	containersData, err := portainerAPIGet(w.URL, w.APIToken, fmt.Sprintf("/api/endpoints/%s/docker/containers/json?all=true", endpointID))
	if err != nil {
		return nil, fmt.Errorf("get containers: %w", err)
	}

	if containers, ok := containersData.([]interface{}); ok {
		result.Total = len(containers)
		for _, c := range containers {
			if container, ok := c.(map[string]interface{}); ok {
				if state, ok := container["State"].(string); ok && state == "running" {
					result.Running++
				} else {
					result.Stopped++
				}
			}
		}
	}

	return result, nil
}

func portainerAPIGet(baseURL, apiKey, path string) (interface{}, error) {
	req, err := http.NewRequest("GET", baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := proxyClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// --- AdGuard Home ---

type AdGuardData struct {
	Queries       int     `json:"queries"`
	Blocked       int     `json:"blocked"`
	Filtered      int     `json:"filtered"`
	AvgTime       float64 `json:"avg_time"`
	BlockedPct    float64 `json:"blocked_percent"`
}

func fetchAdGuardData(w *storage.Widget) (*AdGuardData, error) {
	req, err := http.NewRequest("GET", w.URL+"/control/stats", nil)
	if err != nil {
		return nil, err
	}

	// APIToken format: "username:password"
	if w.APIToken != "" {
		parts := strings.SplitN(w.APIToken, ":", 2)
		if len(parts) == 2 {
			req.SetBasicAuth(parts[0], parts[1])
		}
	}

	resp, err := proxyClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	var stats struct {
		NumDNSQueries           int     `json:"num_dns_queries"`
		NumBlockedFiltering     int     `json:"num_blocked_filtering"`
		NumReplacedSafebrowsing int     `json:"num_replaced_safebrowsing"`
		NumReplacedParental     int     `json:"num_replaced_parental"`
		AvgProcessingTime       float64 `json:"avg_processing_time"`
	}
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, err
	}

	result := &AdGuardData{
		Queries:  stats.NumDNSQueries,
		Blocked:  stats.NumBlockedFiltering,
		Filtered: stats.NumReplacedSafebrowsing + stats.NumReplacedParental,
		AvgTime:  stats.AvgProcessingTime,
	}
	if result.Queries > 0 {
		result.BlockedPct = float64(result.Blocked) / float64(result.Queries) * 100
	}

	return result, nil
}

// --- Jellyfin ---

type JellyfinSession struct {
	UserName     string `json:"user_name"`
	Client       string `json:"client"`
	DeviceName   string `json:"device_name"`
	NowPlaying   string `json:"now_playing,omitempty"`
	ProgressTicks int64 `json:"progress_ticks,omitempty"`
	RuntimeTicks  int64 `json:"runtime_ticks,omitempty"`
	IsPaused     bool   `json:"is_paused"`
}

type JellyfinData struct {
	Movies      int               `json:"movies"`
	Episodes    int               `json:"episodes"`
	OnlineUsers int               `json:"online_users"`
	NowPlaying  int               `json:"now_playing"`
	Status      string            `json:"status"`
	Sessions    []JellyfinSession `json:"sessions"`
}

func fetchJellyfinData(w *storage.Widget) (*JellyfinData, error) {
	result := &JellyfinData{Status: "online", Sessions: []JellyfinSession{}}

	// Helper to make authenticated requests
	doReq := func(path string) ([]byte, error) {
		req, err := http.NewRequest("GET", w.URL+path, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Emby-Token", w.APIToken)
		resp, err := proxyClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("API %s returned %d", path, resp.StatusCode)
		}
		return body, nil
	}

	// 1. Get media counts
	if body, err := doReq("/Items/Counts"); err == nil {
		var counts struct {
			MovieCount   int `json:"MovieCount"`
			EpisodeCount int `json:"EpisodeCount"`
		}
		if json.Unmarshal(body, &counts) == nil {
			result.Movies = counts.MovieCount
			result.Episodes = counts.EpisodeCount
		}
	}

	// 2. Get sessions (online users + now playing)
	body, err := doReq("/Sessions")
	if err != nil {
		return nil, err
	}

	var sessions []struct {
		UserName         string `json:"UserName"`
		Client           string `json:"Client"`
		DeviceName       string `json:"DeviceName"`
		LastActivityDate string `json:"LastActivityDate"`
		NowPlayingItem   *struct {
			Name         string `json:"Name"`
			SeriesName   string `json:"SeriesName"`
			RunTimeTicks int64  `json:"RunTimeTicks"`
		} `json:"NowPlayingItem"`
		PlayState *struct {
			PositionTicks int64 `json:"PositionTicks"`
			IsPaused      bool  `json:"IsPaused"`
		} `json:"PlayState"`
	}
	if err := json.Unmarshal(body, &sessions); err != nil {
		return nil, err
	}

	// Count unique online users (only if playing or active within 5 minutes)
	userSet := make(map[string]bool)
	for _, s := range sessions {
		if s.UserName != "" {
			isActive := s.NowPlayingItem != nil
			if !isActive {
				// Check if LastActivityDate is within 5 minutes
				if t, err := time.Parse(time.RFC3339Nano, s.LastActivityDate); err == nil {
					if time.Since(t) < 5*time.Minute {
						isActive = true
					}
				} else if t, err := time.Parse("2006-01-02T15:04:05.0000000Z", s.LastActivityDate); err == nil {
					if time.Since(t) < 5*time.Minute {
						isActive = true
					}
				}
			}
			if isActive {
				userSet[s.UserName] = true
			}
		}

		session := JellyfinSession{
			UserName:   s.UserName,
			Client:     s.Client,
			DeviceName: s.DeviceName,
		}

		if s.NowPlayingItem != nil {
			result.NowPlaying++
			name := s.NowPlayingItem.Name
			if s.NowPlayingItem.SeriesName != "" {
				name = s.NowPlayingItem.SeriesName + " - " + name
			}
			session.NowPlaying = name
			session.RuntimeTicks = s.NowPlayingItem.RunTimeTicks
			if s.PlayState != nil {
				session.ProgressTicks = s.PlayState.PositionTicks
				session.IsPaused = s.PlayState.IsPaused
			}
		}

		// Only include sessions with an active user
		if s.UserName != "" && userSet[s.UserName] {
			result.Sessions = append(result.Sessions, session)
		}
	}

	result.OnlineUsers = len(userSet)

	if result.NowPlaying == 0 {
		result.Status = "暂无播放"
	} else {
		result.Status = fmt.Sprintf("%d 路播放中", result.NowPlaying)
	}

	return result, nil
}

// --- MoviePilot ---

type MoviePilotData struct {
	MovieSubscribes int    `json:"movie_subscribes"`
	TVSubscribes    int    `json:"tv_subscribes"`
	TotalStorage    string `json:"total_storage"`
	FreeStorage     string `json:"free_storage"`
}

func fetchMoviePilotData(w *storage.Widget) (*MoviePilotData, error) {
	// MoviePilot uses apikey as query param
	baseURL := strings.TrimRight(w.URL, "/")
	url := baseURL + "/api/v1/plugin/HomePage/statistic?apikey=" + w.APIToken

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := proxyClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	var stats struct {
		MovieSubscribes int    `json:"movie_subscribes"`
		TVSubscribes    int    `json:"tv_subscribes"`
		TotalStorage    string `json:"total_storage"`
		FreeStorage     string `json:"free_storage"`
	}
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, err
	}

	return &MoviePilotData{
		MovieSubscribes: stats.MovieSubscribes,
		TVSubscribes:    stats.TVSubscribes,
		TotalStorage:    stats.TotalStorage,
		FreeStorage:     stats.FreeStorage,
	}, nil
}

// --- qBittorrent ---

type QBittorrentData struct {
	Downloading int   `json:"downloading"`
	DlSpeed     int64 `json:"dl_speed"`
	Seeding     int   `json:"seeding"`
	UpSpeed     int64 `json:"up_speed"`
}

func fetchQBittorrentData(w *storage.Widget) (*QBittorrentData, error) {
	// Login first to get SID cookie
	// APIToken format: "username:password"
	username := "admin"
	password := ""
	if w.APIToken != "" {
		parts := strings.SplitN(w.APIToken, ":", 2)
		if len(parts) == 2 {
			username = parts[0]
			password = parts[1]
		}
	}

	loginReq, err := http.NewRequest("POST", w.URL+"/api/v2/auth/login",
		strings.NewReader(fmt.Sprintf("username=%s&password=%s", username, password)))
	if err != nil {
		return nil, err
	}
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	loginResp, err := proxyClient.Do(loginReq)
	if err != nil {
		return nil, err
	}
	defer loginResp.Body.Close()

	// Extract SID cookie
	var sid string
	for _, cookie := range loginResp.Cookies() {
		if cookie.Name == "SID" {
			sid = cookie.Value
			break
		}
	}
	if sid == "" {
		return nil, fmt.Errorf("login failed: no SID cookie")
	}

	result := &QBittorrentData{}

	// Get transfer info
	transferReq, err := http.NewRequest("GET", w.URL+"/api/v2/transfer/info", nil)
	if err != nil {
		return nil, err
	}
	transferReq.AddCookie(&http.Cookie{Name: "SID", Value: sid})

	transferResp, err := proxyClient.Do(transferReq)
	if err != nil {
		return nil, err
	}
	defer transferResp.Body.Close()

	transferBody, _ := io.ReadAll(transferResp.Body)
	var transfer struct {
		DlInfoSpeed int64 `json:"dl_info_speed"`
		UpInfoSpeed int64 `json:"up_info_speed"`
	}
	json.Unmarshal(transferBody, &transfer)
	result.DlSpeed = transfer.DlInfoSpeed
	result.UpSpeed = transfer.UpInfoSpeed

	// Get torrent counts
	torrentsReq, err := http.NewRequest("GET", w.URL+"/api/v2/torrents/info", nil)
	if err != nil {
		return nil, err
	}
	torrentsReq.AddCookie(&http.Cookie{Name: "SID", Value: sid})

	torrentsResp, err := proxyClient.Do(torrentsReq)
	if err != nil {
		return nil, err
	}
	defer torrentsResp.Body.Close()

	torrentsBody, _ := io.ReadAll(torrentsResp.Body)
	var torrents []struct {
		State string `json:"state"`
	}
	json.Unmarshal(torrentsBody, &torrents)

	for _, t := range torrents {
		switch t.State {
		case "downloading", "stalledDL", "forcedDL", "metaDL", "allocating":
			result.Downloading++
		case "uploading", "stalledUP", "forcedUP", "queuedUP":
			result.Seeding++
		}
	}

	return result, nil
}

// --- Hawkeye Self Status ---

type HawkeyeData struct {
	OnlineAgents int `json:"online_agents"`
	TotalAgents  int `json:"total_agents"`
	OnlineProbes int `json:"online_probes"`
	TotalProbes  int `json:"total_probes"`
	Alerts       int `json:"alerts"`
}

func fetchHawkeyeData(db *storage.DB) (*HawkeyeData, error) {
	result := &HawkeyeData{}

	// Count agents
	agents, err := db.GetAllAgents()
	if err == nil {
		result.TotalAgents = len(agents)
		for _, a := range agents {
			if a.Status == "online" {
				result.OnlineAgents++
			}
		}
	}

	// Count probes
	probes, err := db.GetAllProbes()
	if err == nil {
		result.TotalProbes = len(probes)
		for _, p := range probes {
			if p.Enabled {
				result.OnlineProbes++
			}
		}
	}

	// Count unresolved alerts
	events, err := db.GetAlertEvents(false, 1000)
	if err == nil {
		result.Alerts = len(events)
	}

	return result, nil
}

// --- Widget Reorder ---

type reorderWidgetsRequest struct {
	IDs []int64 `json:"ids"`
}

// --- iKuai ---

type OpenClashData struct {
	Version          string  `json:"version"`
	Node             string  `json:"node"`
	NodeType         string  `json:"node_type"`
	TrafficUp        float64 `json:"traffic_up"`        // cumulative upload total, bytes
	TrafficDown      float64 `json:"traffic_down"`      // cumulative download total, bytes
	PingLatency      float64 `json:"ping_latency"`      // ping youtube.com latency, ms
	RemainingTraffic float64 `json:"remaining_traffic"`
}

type IkuaiData struct {
	CPU      float64 `json:"cpu"`
	Clients  int     `json:"clients"`
	Download float64 `json:"download"` // KB/s
	Upload   float64 `json:"upload"`   // KB/s
}

func fetchIkuaiData(w *storage.Widget) (*IkuaiData, error) {
	baseURL := strings.TrimRight(w.URL, "/")
	token := w.APIToken
	result := &IkuaiData{}

	// 1. CPU usage
	var cpuResp struct {
		Results struct {
			SoftirqData []struct {
				Used string `json:"used"`
			} `json:"softirq_data"`
		} `json:"results"`
	}
	if err := ikuaiAPIGet(baseURL, token, "/system/cpufreq", &cpuResp); err != nil {
		return nil, fmt.Errorf("cpu: %w", err)
	}
	var cpuTotal float64
	for _, c := range cpuResp.Results.SoftirqData {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(c.Used, "%"), 64)
		cpuTotal += v
	}
	if len(cpuResp.Results.SoftirqData) > 0 {
		result.CPU = cpuTotal / float64(len(cpuResp.Results.SoftirqData))
	}

	// 2. Online clients (IPv4 + IPv6)
	var ipv4Resp struct {
		Results struct {
			Data []interface{} `json:"data"`
		} `json:"results"`
	}
	if err := ikuaiAPIGet(baseURL, token, "/monitoring/clients-online?limit=500&page=1", &ipv4Resp); err != nil {
		return nil, fmt.Errorf("clients ipv4: %w", err)
	}
	result.Clients = len(ipv4Resp.Results.Data)

	var ipv6Resp struct {
		Results struct {
			Data []interface{} `json:"data"`
		} `json:"results"`
	}
	if err := ikuaiAPIGet(baseURL, token, "/monitoring/clients-ip6-online?limit=500&page=1", &ipv6Resp); err == nil {
		result.Clients += len(ipv6Resp.Results.Data)
	}

	// 3. Traffic rates
	var trafficResp struct {
		Results struct {
			WansStatHistory []struct {
				Interface   string `json:"interface"`
				AvgDownload string `json:"avg_download"`
				AvgUpload   string `json:"avg_upload"`
				Timestamp   int64  `json:"timestamp"`
			} `json:"wans_stat_history"`
		} `json:"results"`
	}
	if err := ikuaiAPIGet(baseURL, token, "/monitoring/interfaces-traffic", &trafficResp); err != nil {
		return nil, fmt.Errorf("traffic: %w", err)
	}
	latestTS := int64(0)
	for _, t := range trafficResp.Results.WansStatHistory {
		if t.Interface == "all" && t.Timestamp > latestTS {
			latestTS = t.Timestamp
			result.Download, _ = strconv.ParseFloat(t.AvgDownload, 64)
			result.Upload, _ = strconv.ParseFloat(t.AvgUpload, 64)
		}
	}

	return result, nil
}

func ikuaiAPIGet(baseURL, token, path string, out interface{}) error {
	req, _ := http.NewRequest("GET", baseURL+"/api/v4.0"+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := proxyClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func reorderWidgets(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req reorderWidgetsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(req.IDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ids required"})
			return
		}
		if err := db.ReorderWidgets(req.IDs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "reordered"})
	}
}

// --- Widget Move (change group only) ---

type moveWidgetRequest struct {
	ID    int64  `json:"id"`
	Group string `json:"widget_group"`
}

func moveWidget(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req moveWidgetRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.ID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
			return
		}
		if err := db.UpdateWidgetGroup(req.ID, req.Group); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "moved"})
	}
}

// --- Widget Group Rename ---

type renameGroupRequest struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

func renameWidgetGroup(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req renameGroupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.OldName == "" || req.NewName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "old_name and new_name required"})
			return
		}
		if req.OldName == req.NewName {
			c.JSON(http.StatusOK, gin.H{"message": "renamed"})
			return
		}
		// Reject if the new name already exists as a different group
		exists, err := db.GroupExists(req.NewName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": "分组名 \"" + req.NewName + "\" 已存在"})
			return
		}
		if err := db.RenameWidgetGroup(req.OldName, req.NewName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "renamed"})
	}
}

// --- Lucky ---

type LuckyData struct {
	CPU           string `json:"cpu"`
	NetInSpeed    string `json:"net_in_speed"`
	NetOutSpeed   string `json:"net_out_speed"`
	RulesCount    int    `json:"rules_count"`
	SubRulesCount int    `json:"sub_rules_count"`
	EnabledCount  int    `json:"enabled_count"`
}

func fetchLuckyData(w *storage.Widget) (*LuckyData, error) {
	baseURL := strings.TrimRight(w.URL, "/")

	// Use APIToken as OpenToken
	openToken := w.APIToken

	// Helper for Lucky API calls
	luckyGet := func(path string) (map[string]interface{}, error) {
		reqURL := baseURL + path
		if strings.Contains(reqURL, "?") {
			reqURL += "&openToken=" + openToken
		} else {
			reqURL += "?openToken=" + openToken
		}

		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			return nil, err
		}

		resp, err := proxyClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}
		return result, nil
	}

	result := &LuckyData{}

	// Get system status
	statusData, err := luckyGet("/api/status")
	if err != nil {
		return nil, fmt.Errorf("get status: %w", err)
	}

	if data, ok := statusData["data"].(map[string]interface{}); ok {
		if cpu, ok := data["usedCPU"].(string); ok {
			result.CPU = cpu
		}
		if netIn, ok := data["lastNetInSpeed"].(float64); ok {
			result.NetInSpeed = formatSpeed(int64(netIn))
		}
		if netOut, ok := data["lastNetOutSpeed"].(float64); ok {
			result.NetOutSpeed = formatSpeed(int64(netOut))
		}
	}

	// Get web service rules
	rulesData, err := luckyGet("/api/webservice/rules")
	if err == nil {
		if ruleList, ok := rulesData["ruleList"].([]interface{}); ok {
			result.RulesCount = len(ruleList)
			for _, r := range ruleList {
				if rule, ok := r.(map[string]interface{}); ok {
					if proxies, ok := rule["ProxyList"].([]interface{}); ok {
						result.SubRulesCount += len(proxies)
						for _, p := range proxies {
							if proxy, ok := p.(map[string]interface{}); ok {
								if enable, ok := proxy["Enable"].(bool); ok && enable {
									result.EnabledCount++
								}
							}
						}
					}
				}
			}
		}
	}

	return result, nil
}

// --- Transmission ---

type TransmissionData struct {
	Downloading int   `json:"downloading"`
	DlSpeed     int64 `json:"dl_speed"`
	Seeding     int   `json:"seeding"`
	UpSpeed     int64 `json:"up_speed"`
	Paused      int   `json:"paused"`
	Total       int   `json:"total"`
}

func fetchTransmissionData(w *storage.Widget) (*TransmissionData, error) {
	// Parse credentials from APIToken (format: "username:password")
	username := ""
	password := ""
	if w.APIToken != "" {
		parts := strings.SplitN(w.APIToken, ":", 2)
		if len(parts) == 2 {
			username = parts[0]
			password = parts[1]
		}
	}

	baseURL := strings.TrimRight(w.URL, "/")
	rpcURL := baseURL + "/transmission/rpc"

	// Session ID for Transmission RPC (needed after first 409 response)
	sessionID := ""

	type rpcRequest struct {
		Method    string                 `json:"method"`
		Arguments map[string]interface{} `json:"arguments,omitempty"`
	}

	// Helper: make an RPC call
	rpcCall := func(method string, args map[string]interface{}) (map[string]interface{}, error) {
		reqBody := rpcRequest{Method: method, Arguments: args}
		bodyBytes, _ := json.Marshal(reqBody)

		req, err := http.NewRequest("POST", rpcURL, strings.NewReader(string(bodyBytes)))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if username != "" {
			req.SetBasicAuth(username, password)
		}
		if sessionID != "" {
			req.Header.Set("X-Transmission-Session-Id", sessionID)
		}

		resp, err := proxyClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		// 409 = need session ID, extract it and retry once
		if resp.StatusCode == 409 {
			newSID := resp.Header.Get("X-Transmission-Session-Id")
			if newSID != "" && newSID != sessionID {
				sessionID = newSID
				// Retry with new session ID
				req2, err := http.NewRequest("POST", rpcURL, strings.NewReader(string(bodyBytes)))
				if err != nil {
					return nil, err
				}
				req2.Header.Set("Content-Type", "application/json")
				if username != "" {
					req2.SetBasicAuth(username, password)
				}
				req2.Header.Set("X-Transmission-Session-Id", sessionID)
				resp2, err := proxyClient.Do(req2)
				if err != nil {
					return nil, err
				}
				defer resp2.Body.Close()
				resp = resp2
			}
		}

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("Transmission RPC returned %d", resp.StatusCode)
		}

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var result struct {
			Arguments map[string]interface{} `json:"arguments"`
			Result    string                 `json:"result"`
		}
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, err
		}
		if result.Result != "success" {
			return nil, fmt.Errorf("RPC failed: %s", result.Result)
		}
		return result.Arguments, nil
	}

	result := &TransmissionData{}

	// Get session stats (speed & totals only — downloading/seeding counts from per-torrent status)
	stats, err := rpcCall("session-stats", nil)
	if err != nil {
		return nil, fmt.Errorf("session-stats: %w", err)
	}
	if s, ok := stats["downloadSpeed"].(float64); ok {
		result.DlSpeed = int64(s)
	}
	if s, ok := stats["uploadSpeed"].(float64); ok {
		result.UpSpeed = int64(s)
	}
	if s, ok := stats["pausedTorrentCount"].(float64); ok {
		result.Paused = int(s)
	}
	if s, ok := stats["torrentCount"].(float64); ok {
		result.Total = int(s)
	}

	// Count downloading/seeding from per-torrent status (activeTorrentCount includes both)
	torrents, err := rpcCall("torrent-get", map[string]interface{}{
		"fields": []string{"status"},
	})
	if err == nil {
		if torrentList, ok := torrents["torrents"].([]interface{}); ok {
			for _, t := range torrentList {
				if torrent, ok := t.(map[string]interface{}); ok {
					if status, ok := torrent["status"].(float64); ok {
						switch int(status) {
						case 3, 4: // download wait, downloading
							result.Downloading++
						case 5, 6: // seed wait, seeding
							result.Seeding++
						}
					}
				}
			}
		}
	}

	return result, nil
}

// --- Home Assistant ---

type HomeAssistantCustomEntity struct {
	EntityID string `json:"entity_id"`
	Label    string `json:"label,omitempty"`
	Unit     string `json:"unit,omitempty"`
}

type HomeAssistantData struct {
	PeopleHome  int                          `json:"people_home"`
	LightsOn    int                          `json:"lights_on"`
	SwitchesOn  int                          `json:"switches_on"`
	TotalStates int                          `json:"total_states"`
	HAStatus    string                       `json:"ha_status"`
	Version     string                       `json:"version,omitempty"`
	Custom      []HomeAssistantCustomResult  `json:"custom,omitempty"`
}

type HomeAssistantCustomResult struct {
	EntityID string `json:"entity_id"`
	Label    string `json:"label"`
	Value    string `json:"value"`
	Unit     string `json:"unit,omitempty"`
}

func fetchHomeAssistantData(w *storage.Widget) (*HomeAssistantData, error) {
	result := &HomeAssistantData{}

	// Parse custom entities from config
	var config struct {
		LinkURL  string                       `json:"link_url,omitempty"`
		Entities []HomeAssistantCustomEntity  `json:"entities,omitempty"`
	}
	if w.Config != "" {
		json.Unmarshal([]byte(w.Config), &config)
	}

	// Get all states
	req, err := http.NewRequest("GET", w.URL+"/api/states", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+w.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := proxyClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	var states []map[string]interface{}
	if err := json.Unmarshal(body, &states); err != nil {
		return nil, fmt.Errorf("parse states: %w", err)
	}

	result.TotalStates = len(states)

	// Create map for quick entity lookup
	entityMap := make(map[string]map[string]interface{})
	for _, s := range states {
		if entityID, ok := s["entity_id"].(string); ok {
			entityMap[entityID] = s
		}
	}

	// Count people at home
	for entityID, s := range entityMap {
		if strings.HasPrefix(entityID, "person.") {
			if state, ok := s["state"].(string); ok && state == "home" {
				result.PeopleHome++
			}
		}
	}

	// Count lights on
	for entityID, s := range entityMap {
		if strings.HasPrefix(entityID, "light.") {
			if state, ok := s["state"].(string); ok && state == "on" {
				result.LightsOn++
			}
		}
	}

	// Count switches on
	for entityID, s := range entityMap {
		if strings.HasPrefix(entityID, "switch.") {
			if state, ok := s["state"].(string); ok && state == "on" {
				result.SwitchesOn++
			}
		}
	}

	result.HAStatus = "online"

	// Try to get version from /api/config
	configReq, _ := http.NewRequest("GET", w.URL+"/api/config", nil)
	configReq.Header.Set("Authorization", "Bearer "+w.APIToken)
	if configResp, err := proxyClient.Do(configReq); err == nil {
		defer configResp.Body.Close()
		if configBody, err := io.ReadAll(configResp.Body); err == nil {
			var cfg struct {
				Version string `json:"version"`
			}
			if json.Unmarshal(configBody, &cfg) == nil {
				result.Version = cfg.Version
			}
		}
	}

	// Resolve custom entities
	for _, ce := range config.Entities {
		s, ok := entityMap[ce.EntityID]
		if !ok {
			result.Custom = append(result.Custom, HomeAssistantCustomResult{
				EntityID: ce.EntityID,
				Label:    ce.Label,
				Value:    "unknown",
				Unit:     ce.Unit,
			})
			continue
		}

		state, _ := s["state"].(string)
		label := ce.Label
		if label == "" {
			// Use friendly_name from attributes
			if attrs, ok := s["attributes"].(map[string]interface{}); ok {
				if fn, ok := attrs["friendly_name"].(string); ok {
					label = fn
				}
			}
			if label == "" {
				label = ce.EntityID
			}
		}

		unit := ce.Unit
		if unit == "" {
			if attrs, ok := s["attributes"].(map[string]interface{}); ok {
				if uom, ok := attrs["unit_of_measurement"].(string); ok {
					unit = uom
				}
			}
		}

		result.Custom = append(result.Custom, HomeAssistantCustomResult{
			EntityID: ce.EntityID,
			Label:    label,
			Value:    state,
			Unit:     unit,
		})
	}

	return result, nil
}

// --- OpenWrt helpers (kept) ---

type OpenWrtData struct {
	Uptime    int64   `json:"uptime"`
	CPULoad   float64 `json:"cpu_load"`
	MemTotal  int64   `json:"mem_total"`
	MemFree   int64   `json:"mem_free"`
	DiskTotal int64   `json:"disk_total"`
	DiskFree  int64   `json:"disk_free"`
}

func fetchOpenWrtData(w *storage.Widget) (interface{}, error) {
	u := strings.TrimRight(w.URL, "/")
	ubusURL := u + "/ubus"

	username := ""
	password := ""
	if w.APIToken != "" {
		parts := strings.SplitN(w.APIToken, ":", 2)
		if len(parts) == 2 {
			username = parts[0]
			password = parts[1]
		}
	}
	if username == "" || password == "" {
		return nil, fmt.Errorf("OpenWrt requires username:password in API Token field")
	}

	// Login
	loginParams := openwrtRPCParams("00000000000000000000000000000000", "session", "login", map[string]string{
		"username": username,
		"password": password,
	})
	loginResp, err := openwrtRPCCall(ubusURL, loginParams)
	if err != nil {
		return nil, fmt.Errorf("ubus login: %w", err)
	}
	sessionToken, err := openwrtParseLogin(loginResp)
	if err != nil {
		return nil, fmt.Errorf("parse login response: %w", err)
	}

	// Fetch system info
	sysParams := openwrtRPCParams(sessionToken, "system", "info", map[string]string{})
	sysResp, err := openwrtRPCCall(ubusURL, sysParams)
	if err != nil {
		return nil, fmt.Errorf("ubus system info: %w", err)
	}
	sysData, err := openwrtParseSystem(sysResp)
	if err != nil {
		return nil, fmt.Errorf("parse system info: %w", err)
	}

	return sysData, nil
}

// --- OpenWrt JSON-RPC helpers ---

type openwrtRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

func openwrtRPCParams(session, ubusObj, method string, params interface{}) openwrtRPCRequest {
	return openwrtRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "call",
		Params:  []interface{}{session, ubusObj, method, params},
	}
}

func openwrtRPCCall(ubusURL string, req openwrtRPCRequest) (map[string]interface{}, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", ubusURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := proxyClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ubus returned %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 200)]))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse ubus response: %w", err)
	}

	// Check JSON-RPC error
	if errObj, ok := result["error"]; ok && errObj != nil {
		errMap, _ := errObj.(map[string]interface{})
		msg, _ := errMap["message"].(string)
		code, _ := errMap["code"].(float64)
		return nil, fmt.Errorf("ubus error %d: %s", int(code), msg)
	}

	return result, nil
}

func openwrtParseLogin(resp map[string]interface{}) (string, error) {
	result, ok := resp["result"]
	if !ok {
		return "", fmt.Errorf("no result in ubus response")
	}
	resultArr, ok := result.([]interface{})
	if !ok || len(resultArr) < 2 {
		return "", fmt.Errorf("unexpected result format")
	}
	sessionObj, ok := resultArr[1].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected session format")
	}
	token, ok := sessionObj["ubus_rpc_session"].(string)
	if !ok || token == "" {
		return "", fmt.Errorf("no ubus_rpc_session in response")
	}
	return token, nil
}

func openwrtParseSystem(resp map[string]interface{}) (*OpenWrtData, error) {
	result, ok := resp["result"]
	if !ok {
		return nil, fmt.Errorf("no result in ubus response")
	}
	resultArr, ok := result.([]interface{})
	if !ok || len(resultArr) < 2 {
		return nil, fmt.Errorf("unexpected result format")
	}
	sysInfo, ok := resultArr[1].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected system info format")
	}

	data := &OpenWrtData{}

	if uptime, ok := sysInfo["uptime"].(float64); ok {
		data.Uptime = int64(uptime)
	}
	if load, ok := sysInfo["load"].([]interface{}); ok && len(load) > 1 {
		if loadVal, ok := load[1].(float64); ok {
			data.CPULoad = loadVal / 65536.0
		}
	}
	// memory
	if mem, ok := sysInfo["memory"].(map[string]interface{}); ok {
		if total, ok := mem["total"].(float64); ok {
			data.MemTotal = int64(total)
		}
		if avail, ok := mem["available"].(float64); ok {
			data.MemFree = int64(avail)
		}
	}
	// disk (root) — ubus returns KiB for disk, convert to bytes
	if root, ok := sysInfo["root"].(map[string]interface{}); ok {
		if total, ok := root["total"].(float64); ok {
			data.DiskTotal = int64(total) * 1024
		}
		if free, ok := root["free"].(float64); ok {
			data.DiskFree = int64(free) * 1024
		}
	}

	return data, nil
}

func formatSpeed(bytesPerSec int64) string {
	if bytesPerSec == 0 {
		return "0 B/s"
	}
	units := []string{"B/s", "KB/s", "MB/s", "GB/s"}
	var i int
	val := float64(bytesPerSec)
	for i = 0; val >= 1024 && i < len(units)-1; i++ {
		val /= 1024
	}
	return fmt.Sprintf("%.0f %s", val, units[i])
}

// --- OpenClash ---

type openClashProxy struct {
	Type    string          `json:"type"`
	Now     string          `json:"now"`
	All     []string        `json:"all"`
	History []struct {
		Time  string `json:"time"`
		Delay int    `json:"delay"`
	} `json:"history"`
}

func fetchOpenClashData(w *storage.Widget) (*OpenClashData, error) {
	baseURL := strings.TrimRight(w.URL, "/")
	secret := w.APIToken
	result := &OpenClashData{}

	var wg sync.WaitGroup
	wg.Add(3)

	// 1. Fetch proxies (concurrent)
	go func() {
		defer wg.Done()
		req, err := http.NewRequest("GET", baseURL+"/proxies", nil)
		if err != nil {
			return
		}
		req.Header.Set("Authorization", "Bearer "+secret)
		resp, err := proxyClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		var proxiesResp map[string]json.RawMessage
		if err := json.Unmarshal(body, &proxiesResp); err != nil {
			return
		}
		proxiesData, ok := proxiesResp["proxies"]
		if !ok {
			return
		}
		var allProxies map[string]openClashProxy
		if err := json.Unmarshal(proxiesData, &allProxies); err != nil {
			return
		}
		if global, ok := allProxies["良心云"]; ok {
			result.Node = global.Now
			result.NodeType = global.Type
			for _, name := range global.All {
				if strings.Contains(name, "剩余流量") {
					parts := strings.Split(name, "：")
					if len(parts) == 2 {
						valStr := strings.TrimSuffix(strings.TrimSpace(parts[1]), " GB")
						result.RemainingTraffic, _ = strconv.ParseFloat(valStr, 64)
					}
				}
			}
		}
		// Measure real-time delay by triggering a Clash url-test on the
		//   current node (GET /proxies/{name}/delay).  Falls back to the
		//   node's cached history value if the test times out or fails.
		if global, ok := allProxies["良心云"]; ok && global.Now != "" {
			delayURL := baseURL + "/proxies/" + url.PathEscape(global.Now) + "/delay?url=https://www.gstatic.com/generate_204&timeout=5000"
			req, _ := http.NewRequest("GET", delayURL, nil)
			req.Header.Set("Authorization", "Bearer "+secret)
			dctx, dcancel := context.WithTimeout(context.Background(), 4000*time.Millisecond)
			resp, err := proxyClient.Do(req.WithContext(dctx))
			dcancel()
			if err == nil {
				defer resp.Body.Close()
				var dr struct{ Delay int64 `json:"delay"` }
				if json.NewDecoder(resp.Body).Decode(&dr) == nil && dr.Delay > 0 {
					result.PingLatency = float64(dr.Delay)
				}
			}
			// Fallback: use latest non-zero history value
			if result.PingLatency == 0 {
				if node, ok := allProxies[global.Now]; ok {
					for _, h := range node.History {
						if h.Delay > 0 {
							result.PingLatency = float64(h.Delay)
							break
						}
					}
				}
			}
		}
	}()

	// 2. Quick version check (concurrent)
	go func() {
		defer wg.Done()
		req, _ := http.NewRequest("GET", baseURL+"/version", nil)
		req.Header.Set("Authorization", "Bearer "+secret)
		resp, err := proxyClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		var verData struct {
			Version string `json:"version"`
		}
		verBody, _ := io.ReadAll(resp.Body)
		json.Unmarshal(verBody, &verData)
		result.Version = verData.Version
	}()

	// 3. Cumulative traffic — best-effort, 1500ms deadline (concurrent)
	//    The /traffic endpoint is a newline-delimited JSON stream (~1s interval).
	//    Take the first event's upTotal/downTotal as cumulative totals (in bytes).
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		defer cancel()
		trafficReq, _ := http.NewRequestWithContext(ctx, "GET", baseURL+"/traffic", nil)
		trafficReq.Header.Set("Authorization", "Bearer "+secret)
		trafficResp, err := proxyClient.Do(trafficReq)
		if err == nil {
			defer trafficResp.Body.Close()
			scanner := bufio.NewScanner(trafficResp.Body)
			var td struct {
				UpTotal   int64 `json:"upTotal"`
				DownTotal int64 `json:"downTotal"`
			}
			if scanner.Scan() {
				json.Unmarshal(scanner.Bytes(), &td)
				result.TrafficUp = float64(td.UpTotal)
				result.TrafficDown = float64(td.DownTotal)
			}
			log.Printf("[openclash-traffic] upTotal=%d downTotal=%d", td.UpTotal, td.DownTotal)
		} else {
			log.Printf("[openclash-traffic] request failed: %v", err)
		}
	}()

	wg.Wait()

	return result, nil
}

// --- OpenClash node switch ---

func getOpenClashNodes(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid widget id"})
			return
		}
		w, err := db.GetWidget(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "widget not found"})
			return
		}

		baseURL := strings.TrimRight(w.URL, "/")
		secret := w.APIToken

		req, _ := http.NewRequest("GET", baseURL+"/proxies/%E8%89%AF%E5%BF%83%E4%BA%91", nil)
		req.Header.Set("Authorization", "Bearer "+secret)
		resp, err := proxyClient.Do(req)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var result struct {
			Now string   `json:"now"`
			All []string `json:"all"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "parse error"})
			return
		}

		// Filter: only real proxy nodes
		nodes := []string{}
		for _, name := range result.All {
			if name == "DIRECT" || name == "REJECT" || name == "自动选择" || name == "故障转移" {
				continue
			}
			if strings.Contains(name, "剩余流量") || strings.Contains(name, "重置剩余") || strings.Contains(name, "套餐到期") {
				continue
			}
			nodes = append(nodes, name)
		}

		c.JSON(http.StatusOK, gin.H{
			"current": result.Now,
			"nodes":   nodes,
		})
	}
}

func switchOpenClashNode(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid widget id"})
			return
		}
		w, err := db.GetWidget(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "widget not found"})
			return
		}

		var req struct {
			Node string `json:"node"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Node == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "node is required"})
			return
		}

		baseURL := strings.TrimRight(w.URL, "/")
		secret := w.APIToken
		payload, _ := json.Marshal(map[string]string{"name": req.Node})

		httpReq, _ := http.NewRequest("PUT", baseURL+"/proxies/%E8%89%AF%E5%BF%83%E4%BA%91", bytes.NewReader(payload))
		httpReq.Header.Set("Authorization", "Bearer "+secret)
		httpReq.Header.Set("Content-Type", "application/json")
		resp, err := proxyClient.Do(httpReq)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		resp.Body.Close()

		if resp.StatusCode != 204 {
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("unexpected status %d", resp.StatusCode)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "switched", "node": req.Node})
	}
}

// --- OpenClash SSH control ---

type openclashSSHConfig struct {
	SSHHost     string `json:"ssh_host"`
	SSHPort     int    `json:"ssh_port"`
	SSHUser     string `json:"ssh_user"`
	SSHPassword string `json:"ssh_password"`
}

func sshExec(host string, port int, user, password, cmd string) (string, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), config)
	if err != nil {
		return "", fmt.Errorf("SSH 连接失败: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建 SSH 会话失败: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(output), fmt.Errorf("命令执行失败: %w", err)
	}
	return string(output), nil
}

func openclashControl(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid widget id"})
			return
		}
		w, err := db.GetWidget(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "widget not found"})
			return
		}

		var req struct {
			Action string `json:"action"` // start / stop / restart
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		if req.Action != "start" && req.Action != "stop" && req.Action != "restart" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action must be start, stop, or restart"})
			return
		}

		// Parse SSH config from widget config
		var sshCfg openclashSSHConfig
		if w.Config == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "未配置 SSH 连接信息，请先在组件设置中填写"})
			return
		}
		if err := json.Unmarshal([]byte(w.Config), &sshCfg); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "SSH 配置格式错误: " + err.Error()})
			return
		}
		if sshCfg.SSHHost == "" || sshCfg.SSHUser == "" || sshCfg.SSHPassword == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "SSH 配置不完整，需要填写主机、用户名和密码"})
			return
		}
		if sshCfg.SSHPort == 0 {
			sshCfg.SSHPort = 22
		}

		output, err := sshExec(sshCfg.SSHHost, sshCfg.SSHPort, sshCfg.SSHUser, sshCfg.SSHPassword,
			fmt.Sprintf("/etc/init.d/openclash %s", req.Action))
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "output": output})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "ok", "action": req.Action, "output": output})
	}
}

func openclashStatus(db *storage.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid widget id"})
			return
		}
		w, err := db.GetWidget(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "widget not found"})
			return
		}

		// Check if Mihomo API is reachable
		baseURL := strings.TrimRight(w.URL, "/")
		req, _ := http.NewRequest("GET", baseURL+"/version", nil)
		if w.APIToken != "" {
			req.Header.Set("Authorization", "Bearer "+w.APIToken)
		}

		resp, err := proxyClient.Do(req)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"running": false})
			return
		}
		resp.Body.Close()

		running := resp.StatusCode == 200
		c.JSON(http.StatusOK, gin.H{"running": running})
	}
}