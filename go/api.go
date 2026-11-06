package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxDescriptionRunes = 128
	maxClientRunes      = 64
	maxLeaseSeconds     = 86400 * 365
	maxRequestBody      = 1 << 20
)

// API 聚合所有 HTTP 依赖。
type API struct {
	cfg     *Config
	store   *Store
	mgr     *UPnPManager
	started time.Time
}

// ---------- 传输层结构 ----------

type mappingItem struct {
	ID             string `json:"id"`
	Description    string `json:"description"`
	Protocol       string `json:"protocol"`
	ExternalPort   int    `json:"external_port"`
	InternalPort   int    `json:"internal_port"`
	InternalClient string `json:"internal_client"`
	LeaseTime      int    `json:"lease_time"`
	Active         bool   `json:"active"`
	Managed        bool   `json:"managed"`
	CreatedAt      string `json:"created_at"`
}

type mappingListResponse struct {
	Connected  bool          `json:"connected"`
	LanIP      string        `json:"lan_ip"`
	GatewayIP  string        `json:"gateway_ip"`
	ExternalIP string        `json:"external_ip"`
	Error      string        `json:"error"`
	Mappings   []mappingItem `json:"mappings"`
}

type statusResponse struct {
	Connected    bool   `json:"connected"`
	GatewayIP    string `json:"gateway_ip"`
	ExternalIP   string `json:"external_ip"`
	ModelName    string `json:"model_name"`
	Manufacturer string `json:"manufacturer"`
	ServiceType  string `json:"service_type"`
	Error        string `json:"error"`
	Version      string `json:"version"`
	LanIP        string `json:"lan_ip"`
}

type mappingCreateRequest struct {
	Description    string `json:"description"`
	Protocol       string `json:"protocol"`
	ExternalPort   int    `json:"external_port"`
	InternalPort   int    `json:"internal_port"`
	InternalClient string `json:"internal_client"`
	LeaseTime      int    `json:"lease_time"`
}

// ---------- 路由注册 ----------

func (a *API) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", a.handleHealth)
	mux.HandleFunc("GET /api/status", a.handleStatus)
	mux.HandleFunc("POST /api/discover", a.handleDiscover)
	mux.HandleFunc("GET /api/mappings", a.handleListMappings)
	mux.HandleFunc("POST /api/mappings", a.handleCreateMapping)
	mux.HandleFunc("DELETE /api/mappings/port/{port}/{protocol}", a.handleDeleteByPort)
	mux.HandleFunc("DELETE /api/mappings/{id}", a.handleDeleteMapping)
	mux.HandleFunc("POST /api/mappings/{id}/enable", a.handleEnableMapping)
}

// ---------- 处理器 ----------

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": version,
		"uptime":  int(time.Since(a.started).Seconds()),
	})
}

func (a *API) handleStatus(w http.ResponseWriter, r *http.Request) {
	force := r.URL.Query().Get("force") == "true"
	info := a.mgr.Status(r.Context(), force)
	writeJSON(w, http.StatusOK, a.statusBody(info))
}

func (a *API) handleDiscover(w http.ResponseWriter, r *http.Request) {
	info := a.mgr.Status(r.Context(), true)
	writeJSON(w, http.StatusOK, a.statusBody(info))
}

func (a *API) statusBody(info GatewayStatus) statusResponse {
	return statusResponse{
		Connected:    info.Connected,
		GatewayIP:    info.GatewayIP,
		ExternalIP:   info.ExternalIP,
		ModelName:    info.ModelName,
		Manufacturer: info.Manufacturer,
		ServiceType:  info.ServiceType,
		Error:        info.Error,
		Version:      version,
		LanIP:        detectLANIP(),
	}
}

func (a *API) handleListMappings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	managed := a.store.All()
	info := a.mgr.Status(ctx, false)

	resp := mappingListResponse{
		Connected:  info.Connected,
		LanIP:      detectLANIP(),
		GatewayIP:  info.GatewayIP,
		ExternalIP: info.ExternalIP,
		Error:      info.Error,
		Mappings:   []mappingItem{},
	}

	if !info.Connected {
		for _, m := range managed {
			resp.Mappings = append(resp.Mappings, toItem(m, false))
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	routerEntries, enumErr := a.mgr.ListMappings(ctx)
	enumerateOK := enumErr == nil

	routerKeys := make(map[string]struct{}, len(routerEntries))
	for _, entry := range routerEntries {
		routerKeys[portKey(int(entry.ExternalPort), entry.Protocol)] = struct{}{}
	}
	managedKeys := make(map[string]struct{}, len(managed))
	for _, m := range managed {
		managedKeys[portKey(m.ExternalPort, m.Protocol)] = struct{}{}
	}

	for _, m := range managed {
		_, active := routerKeys[portKey(m.ExternalPort, m.Protocol)]
		if !active && !enumerateOK {
			// 枚举失败时退化为逐个精确查询，尽量给出准确的生效状态。
			if specific, err := a.mgr.GetSpecific(ctx, m.ExternalPort, m.Protocol); err == nil && specific != nil {
				active = true
			}
		}
		resp.Mappings = append(resp.Mappings, toItem(m, active))
	}

	for _, entry := range routerEntries {
		if _, exists := managedKeys[portKey(int(entry.ExternalPort), entry.Protocol)]; exists {
			continue
		}
		resp.Mappings = append(resp.Mappings, fromRouter(entry))
	}

	writeJSON(w, http.StatusOK, resp)
}

func (a *API) handleCreateMapping(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var payload mappingCreateRequest
	if err := decodeJSON(w, r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "请求内容不是合法的 JSON")
		return
	}

	if detail := validateCreate(payload); detail != "" {
		writeError(w, http.StatusUnprocessableEntity, detail)
		return
	}
	payload.Protocol = strings.ToUpper(strings.TrimSpace(payload.Protocol))
	if payload.Protocol == "" {
		payload.Protocol = "TCP"
	}

	internalClient := strings.TrimSpace(payload.InternalClient)
	if internalClient == "" {
		internalClient = detectLANIP()
	}
	if internalClient == "" {
		writeError(w, http.StatusBadRequest, "无法确定本机局域网 IP，请手动填写内部 IP")
		return
	}

	existing, err := a.mgr.GetSpecific(ctx, payload.ExternalPort, payload.Protocol)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if existing != nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("外部端口 %d/%s 已被占用", payload.ExternalPort, payload.Protocol))
		return
	}

	description := strings.TrimSpace(payload.Description)
	if description == "" {
		description = fmt.Sprintf("端口 %d", payload.ExternalPort)
	}

	if err := a.mgr.AddMapping(ctx, payload.ExternalPort, payload.Protocol, payload.InternalPort, internalClient, description, payload.LeaseTime); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	entry := Mapping{
		ID:             randomID(),
		Description:    description,
		Protocol:       payload.Protocol,
		ExternalPort:   payload.ExternalPort,
		InternalPort:   payload.InternalPort,
		InternalClient: internalClient,
		LeaseTime:      payload.LeaseTime,
		CreatedAt:      nowISO(),
	}
	if err := a.store.Add(entry); err != nil {
		// 路由器侧已生效，本地落盘失败只记日志，仍向用户返回成功，避免重复添加。
		slog.Warn("保存端口映射配置失败", "err", err)
	}
	writeJSON(w, http.StatusCreated, toItem(entry, true))
}

func (a *API) handleDeleteMapping(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	entry, ok := a.store.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "端口映射不存在")
		return
	}
	if err := a.mgr.DeleteMapping(r.Context(), entry.ExternalPort, entry.Protocol); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	a.store.Remove(id)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleDeleteByPort(w http.ResponseWriter, r *http.Request) {
	port, err := strconv.Atoi(r.PathValue("port"))
	if err != nil || port < 1 || port > 65535 {
		writeError(w, http.StatusBadRequest, "外部端口必须是 1-65535 的整数")
		return
	}
	protocol := strings.ToUpper(r.PathValue("protocol"))
	if protocol != "TCP" && protocol != "UDP" {
		writeError(w, http.StatusBadRequest, "协议必须是 TCP 或 UDP")
		return
	}
	if err := a.mgr.DeleteMapping(r.Context(), port, protocol); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	a.store.RemoveByPort(port, protocol)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleEnableMapping(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	entry, ok := a.store.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "端口映射不存在")
		return
	}
	if err := a.mgr.AddMapping(r.Context(), entry.ExternalPort, entry.Protocol, entry.InternalPort, entry.InternalClient, entry.Description, entry.LeaseTime); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toItem(entry, true))
}

// ---------- 辅助函数 ----------

func validateCreate(payload mappingCreateRequest) string {
	protocol := strings.ToUpper(strings.TrimSpace(payload.Protocol))
	if protocol != "" && protocol != "TCP" && protocol != "UDP" {
		return "协议必须是 TCP 或 UDP"
	}
	if payload.ExternalPort < 1 || payload.ExternalPort > 65535 {
		return "外部端口必须是 1-65535 的整数"
	}
	if payload.InternalPort < 1 || payload.InternalPort > 65535 {
		return "内部端口必须是 1-65535 的整数"
	}
	if utf8.RuneCountInString(payload.Description) > maxDescriptionRunes {
		return fmt.Sprintf("描述最多 %d 个字符", maxDescriptionRunes)
	}
	if utf8.RuneCountInString(strings.TrimSpace(payload.InternalClient)) > maxClientRunes {
		return fmt.Sprintf("内部 IP 最多 %d 个字符", maxClientRunes)
	}
	if payload.LeaseTime < 0 || payload.LeaseTime > maxLeaseSeconds {
		return "租期必须是 0 到 31536000 秒（0 表示永久）"
	}
	return ""
}

func toItem(m Mapping, active bool) mappingItem {
	return mappingItem{
		ID:             m.ID,
		Description:    m.Description,
		Protocol:       m.Protocol,
		ExternalPort:   m.ExternalPort,
		InternalPort:   m.InternalPort,
		InternalClient: m.InternalClient,
		LeaseTime:      m.LeaseTime,
		Active:         active,
		Managed:        true,
		CreatedAt:      m.CreatedAt,
	}
}

func fromRouter(entry PortMapping) mappingItem {
	return mappingItem{
		ID:             "ext-" + randomID(),
		Description:    entry.Description,
		Protocol:       entry.Protocol,
		ExternalPort:   int(entry.ExternalPort),
		InternalPort:   int(entry.InternalPort),
		InternalClient: entry.InternalClient,
		LeaseTime:      int(entry.LeaseDuration),
		Active:         true,
		Managed:        false,
		CreatedAt:      "",
	}
}

func portKey(port int, protocol string) string {
	return strconv.Itoa(port) + "/" + strings.ToUpper(protocol)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	// 不启用 DisallowUnknownFields：前端比后端先升级时不应被旧服务拒绝。
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody))
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]string{"detail": detail})
}
