package handler

import (
	"net/http"
	"strings"

	"github.com/songhuang/flowpartner/backend/internal/response"
	"github.com/songhuang/flowpartner/backend/internal/snapshot"
)

// SnapshotHandler 提供快照列表与详情查询（还原/手动快照走 WebSocket 指令）。
type SnapshotHandler struct {
	mgr *snapshot.Manager
}

// NewSnapshotHandler 创建快照 HTTP 处理器。
func NewSnapshotHandler(mgr *snapshot.Manager) *SnapshotHandler {
	return &SnapshotHandler{mgr: mgr}
}

// Handle 分发：
//   - GET /api/snapshots          → 完整快照列表（仅 complete: true）
//   - GET /api/snapshots/{id}     → 单个快照详情（含受保护文件清单，供还原确认框）
func (h *SnapshotHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/snapshots")
	rest = strings.TrimPrefix(rest, "/")
	if rest == "" {
		list, err := h.mgr.ListSnapshots()
		if err != nil {
			response.WriteJSON(w, http.StatusInternalServerError,
				response.Error(response.CodeInternalError, "读取快照列表失败: "+err.Error()))
			return
		}
		response.WriteJSON(w, http.StatusOK, response.Success(map[string]interface{}{"snapshots": list}))
		return
	}

	detail, err := h.mgr.GetSnapshotDetail(rest)
	if err != nil {
		response.WriteJSON(w, http.StatusNotFound,
			response.Error(response.CodeInvalidParam, err.Error()))
		return
	}
	response.WriteJSON(w, http.StatusOK, response.Success(detail))
}
