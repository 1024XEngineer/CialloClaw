package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"cialloclaw.local/backend/internal/model"
	"cialloclaw.local/backend/internal/store"
)

type Server struct {
	addr       string
	store      *store.Store
	httpServer *http.Server
}

func New(addr string, st *store.Store) *Server {
	s := &Server{addr: addr, store: st}
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.routes(),
	}
	return s
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	h := &handler{store: s.store}

	mux.HandleFunc("GET /api/bootstrap", h.handleBootstrap)
	mux.HandleFunc("GET /api/scenes", h.handleScenes)
	mux.HandleFunc("GET /api/scenes/{id}", h.handleSceneDetail)
	mux.HandleFunc("GET /api/weather/current", h.handleWeatherCurrent)
	mux.HandleFunc("GET /api/situation/{scene}", h.handleSituation)
	mux.HandleFunc("POST /api/action/proceed", h.handleProceed)
	mux.HandleFunc("POST /api/action/risk-preview", h.handleRiskPreview)
	mux.HandleFunc("POST /api/action/authorize", h.handleAuthorize)
	mux.HandleFunc("GET /api/memory", h.handleMemory)
	mux.HandleFunc("GET /api/logs", h.handleLogs)

	return s.loggingMiddleware(s.corsMiddleware(mux))
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

type handler struct {
	store *store.Store
}

type actionRequest struct {
	Scene  string `json:"scene"`
	Action string `json:"action"`
	Depth  string `json:"depth"`
}

func (h *handler) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	bootstrap := h.store.Bootstrap()
	respondOK(w, http.StatusOK, bootstrap, "默认进入多云状态，桌面已准备就绪。")
}

func (h *handler) handleScenes(w http.ResponseWriter, r *http.Request) {
	respondOK(w, http.StatusOK, h.store.Scenes(), "场景列表已就绪。")
}

func (h *handler) handleSceneDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	detail, ok := h.store.SceneDetail(id)
	if !ok {
		respondError(w, http.StatusNotFound, fmt.Sprintf("未找到场景：%s", id))
		return
	}
	respondOK(w, http.StatusOK, detail, detail.StoryLead)
}

func (h *handler) handleWeatherCurrent(w http.ResponseWriter, r *http.Request) {
	sceneID := r.URL.Query().Get("scene")
	weather, ok := h.store.Weather(sceneID)
	if !ok {
		respondError(w, http.StatusNotFound, fmt.Sprintf("未找到天气：%s", sceneID))
		return
	}
	respondOK(w, http.StatusOK, weather, weather.Hint)
}

func (h *handler) handleSituation(w http.ResponseWriter, r *http.Request) {
	sceneID := r.PathValue("scene")
	situation, ok := h.store.Situation(sceneID)
	if !ok {
		respondError(w, http.StatusNotFound, fmt.Sprintf("未找到态势图：%s", sceneID))
		return
	}
	respondOK(w, http.StatusOK, situation, situation.Summary)
}

func (h *handler) handleProceed(w http.ResponseWriter, r *http.Request) {
	var req actionRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	outcome, ok := h.store.Propose(req.Scene, req.Action)
	if !ok {
		respondError(w, http.StatusNotFound, "未找到可推进的动作")
		return
	}
	respondOK(w, http.StatusOK, outcome, outcome.Message)
}

func (h *handler) handleRiskPreview(w http.ResponseWriter, r *http.Request) {
	var req actionRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	preview, ok := h.store.RiskPreview(req.Scene, req.Action)
	if !ok {
		respondError(w, http.StatusNotFound, "未找到雷暴预演数据")
		return
	}
	respondOK(w, http.StatusOK, preview, preview.Summary)
}

func (h *handler) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	var req actionRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, ok := h.store.Authorize(req.Scene, req.Action, req.Depth)
	if !ok {
		respondError(w, http.StatusNotFound, "未找到授权目标")
		return
	}
	respondOK(w, http.StatusOK, result, result.Message)
}

func (h *handler) handleMemory(w http.ResponseWriter, r *http.Request) {
	memory := h.store.Memory()
	respondOK(w, http.StatusOK, memory, memory.Note)
}

func (h *handler) handleLogs(w http.ResponseWriter, r *http.Request) {
	logs := h.store.Logs()
	respondOK(w, http.StatusOK, logs, "最近执行日志已返回。")
}

func decodeJSON(r *http.Request, dst interface{}) error {
	if r.Body == nil {
		return nil
	}
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("JSON 解析失败：%w", err)
	}
	return nil
}

func respondOK(w http.ResponseWriter, status int, data interface{}, message string) {
	writeJSON(w, status, model.APIResponse{OK: true, Message: message, Data: data})
}

func respondError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, model.APIResponse{OK: false, Message: message})
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		log.Printf("write json failed: %v", err)
	}
}
