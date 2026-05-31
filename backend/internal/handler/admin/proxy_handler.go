package admin

import (
	"context"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ProxyHandler handles admin proxy management
type ProxyHandler struct {
	adminService service.AdminService
}

// NewProxyHandler creates a new admin proxy handler
func NewProxyHandler(adminService service.AdminService) *ProxyHandler {
	return &ProxyHandler{
		adminService: adminService,
	}
}

// CreateProxyRequest represents create proxy request
type CreateProxyRequest struct {
	Name              string `json:"name" binding:"required"`
	Protocol          string `json:"protocol" binding:"required,oneof=http https socks5 socks5h"`
	Host              string `json:"host" binding:"required"`
	Port              int    `json:"port" binding:"required,min=1,max=65535"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	Source            string `json:"source"`
	ProxyType         string `json:"proxy_type"`
	Provider          string `json:"provider"`
	Region            string `json:"region"`
	ExitIP            string `json:"exit_ip"`
	QualityStatus     string `json:"quality_status"`
	MaxBoundAccounts  *int   `json:"max_bound_accounts"`
	MaxActiveAccounts *int   `json:"max_active_accounts"`
	Weight            int    `json:"weight"`
}

// UpdateProxyRequest represents update proxy request
type UpdateProxyRequest struct {
	Name              string `json:"name"`
	Protocol          string `json:"protocol" binding:"omitempty,oneof=http https socks5 socks5h"`
	Host              string `json:"host"`
	Port              int    `json:"port" binding:"omitempty,min=1,max=65535"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	Status            string `json:"status" binding:"omitempty,oneof=active inactive"`
	Source            string `json:"source"`
	ProxyType         string `json:"proxy_type"`
	Provider          string `json:"provider"`
	Region            string `json:"region"`
	ExitIP            string `json:"exit_ip"`
	QualityStatus     string `json:"quality_status"`
	MaxBoundAccounts  *int   `json:"max_bound_accounts"`
	MaxActiveAccounts *int   `json:"max_active_accounts"`
	Weight            *int   `json:"weight"`
}

// List handles listing all proxies with pagination
// GET /api/v1/admin/proxies
func (h *ProxyHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	protocol := c.Query("protocol")
	status := c.Query("status")
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort_by", "id")
	sortOrder := c.DefaultQuery("sort_order", "desc")
	// 标准化和验证 search 参数
	search = strings.TrimSpace(search)
	if len(search) > 100 {
		search = search[:100]
	}

	proxies, total, err := h.adminService.ListProxiesWithAccountCount(c.Request.Context(), page, pageSize, protocol, status, search, sortBy, sortOrder)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.AdminProxyWithAccountCount, 0, len(proxies))
	for i := range proxies {
		out = append(out, *dto.ProxyWithAccountCountFromServiceAdmin(&proxies[i]))
	}
	response.Paginated(c, out, total, page, pageSize)
}

// GetAll handles getting all active proxies without pagination
// GET /api/v1/admin/proxies/all
// Optional query param: with_count=true to include account count per proxy
func (h *ProxyHandler) GetAll(c *gin.Context) {
	withCount := c.Query("with_count") == "true"

	if withCount {
		proxies, err := h.adminService.GetAllProxiesWithAccountCount(c.Request.Context())
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		out := make([]dto.AdminProxyWithAccountCount, 0, len(proxies))
		for i := range proxies {
			out = append(out, *dto.ProxyWithAccountCountFromServiceAdmin(&proxies[i]))
		}
		response.Success(c, out)
		return
	}

	proxies, err := h.adminService.GetAllProxies(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.AdminProxy, 0, len(proxies))
	for i := range proxies {
		out = append(out, *dto.ProxyFromServiceAdmin(&proxies[i]))
	}
	response.Success(c, out)
}

// GetByID handles getting a proxy by ID
// GET /api/v1/admin/proxies/:id
func (h *ProxyHandler) GetByID(c *gin.Context) {
	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}

	proxy, err := h.adminService.GetProxy(c.Request.Context(), proxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.ProxyFromServiceAdmin(proxy))
}

// Create handles creating a new proxy
// POST /api/v1/admin/proxies
func (h *ProxyHandler) Create(c *gin.Context) {
	var req CreateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	executeAdminIdempotentJSON(c, "admin.proxies.create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		proxy, err := h.adminService.CreateProxy(ctx, &service.CreateProxyInput{
			Name:              strings.TrimSpace(req.Name),
			Protocol:          strings.TrimSpace(req.Protocol),
			Host:              strings.TrimSpace(req.Host),
			Port:              req.Port,
			Username:          strings.TrimSpace(req.Username),
			Password:          strings.TrimSpace(req.Password),
			Source:            strings.TrimSpace(req.Source),
			ProxyType:         strings.TrimSpace(req.ProxyType),
			Provider:          strings.TrimSpace(req.Provider),
			Region:            strings.TrimSpace(req.Region),
			ExitIP:            strings.TrimSpace(req.ExitIP),
			QualityStatus:     strings.TrimSpace(req.QualityStatus),
			MaxBoundAccounts:  req.MaxBoundAccounts,
			MaxActiveAccounts: req.MaxActiveAccounts,
			Weight:            req.Weight,
		})
		if err != nil {
			return nil, err
		}
		return dto.ProxyFromServiceAdmin(proxy), nil
	})
}

// Update handles updating a proxy
// PUT /api/v1/admin/proxies/:id
func (h *ProxyHandler) Update(c *gin.Context) {
	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}

	var req UpdateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	proxy, err := h.adminService.UpdateProxy(c.Request.Context(), proxyID, &service.UpdateProxyInput{
		Name:              strings.TrimSpace(req.Name),
		Protocol:          strings.TrimSpace(req.Protocol),
		Host:              strings.TrimSpace(req.Host),
		Port:              req.Port,
		Username:          strings.TrimSpace(req.Username),
		Password:          strings.TrimSpace(req.Password),
		Status:            strings.TrimSpace(req.Status),
		Source:            strings.TrimSpace(req.Source),
		ProxyType:         strings.TrimSpace(req.ProxyType),
		Provider:          strings.TrimSpace(req.Provider),
		Region:            strings.TrimSpace(req.Region),
		ExitIP:            strings.TrimSpace(req.ExitIP),
		QualityStatus:     strings.TrimSpace(req.QualityStatus),
		MaxBoundAccounts:  req.MaxBoundAccounts,
		MaxActiveAccounts: req.MaxActiveAccounts,
		Weight:            req.Weight,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.ProxyFromServiceAdmin(proxy))
}

// Delete handles deleting a proxy
// DELETE /api/v1/admin/proxies/:id
func (h *ProxyHandler) Delete(c *gin.Context) {
	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}

	err = h.adminService.DeleteProxy(c.Request.Context(), proxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Proxy deleted successfully"})
}

// BatchDelete handles batch deleting proxies
// POST /api/v1/admin/proxies/batch-delete
func (h *ProxyHandler) BatchDelete(c *gin.Context) {
	type BatchDeleteRequest struct {
		IDs []int64 `json:"ids" binding:"required,min=1"`
	}

	var req BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.adminService.BatchDeleteProxies(c.Request.Context(), req.IDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// Test handles testing proxy connectivity
// POST /api/v1/admin/proxies/:id/test
func (h *ProxyHandler) Test(c *gin.Context) {
	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}

	result, err := h.adminService.TestProxy(c.Request.Context(), proxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// CheckQuality handles checking proxy quality across common AI targets.
// POST /api/v1/admin/proxies/:id/quality-check
func (h *ProxyHandler) CheckQuality(c *gin.Context) {
	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}

	result, err := h.adminService.CheckProxyQuality(c.Request.Context(), proxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// BatchHealthCheck handles testing multiple proxies.
// POST /api/v1/admin/proxies/batch-health-check
func (h *ProxyHandler) BatchHealthCheck(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.adminService.BatchHealthCheckProxies(c.Request.Context(), req.IDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ProxyHandler) PreviewImport(c *gin.Context) {
	var req service.ProxyImportPreviewInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.adminService.PreviewProxyImport(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ProxyHandler) ConfirmImport(c *gin.Context) {
	var req service.ProxyImportConfirmInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.adminService.ConfirmProxyImport(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ProxyHandler) ListProxyRelationships(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	relationships, total, err := h.adminService.ListProxyRelationships(
		c.Request.Context(),
		page,
		pageSize,
		c.Query("platform"),
		c.Query("status"),
		strings.TrimSpace(c.Query("search")),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, relationships, total, page, pageSize)
}

func (h *ProxyHandler) ReassignAccountProxy(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	result, err := h.adminService.ReassignAccountProxy(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ProxyHandler) RestoreAccountProxyHistory(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	result, err := h.adminService.RestoreAccountProxyHistory(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ProxyHandler) GetAccountProxyHistory(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	result, err := h.adminService.GetAccountProxyHistory(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ProxyHandler) GetProxyDispatchSettings(c *gin.Context) {
	result, err := h.adminService.GetProxyDispatchSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ProxyHandler) UpdateProxyDispatchSettings(c *gin.Context) {
	var req service.ProxyDispatchSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.adminService.UpdateProxyDispatchSettings(c.Request.Context(), &req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ProxyHandler) ListProxySubscriptions(c *gin.Context) {
	result, err := h.adminService.ListProxySubscriptionSources(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ProxyHandler) CreateProxySubscription(c *gin.Context) {
	var req service.ProxySubscriptionSourceInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.adminService.CreateProxySubscriptionSource(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ProxyHandler) UpdateProxySubscription(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid subscription ID")
		return
	}
	var req service.ProxySubscriptionSourceInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.adminService.UpdateProxySubscriptionSource(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ProxyHandler) DeleteProxySubscription(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid subscription ID")
		return
	}
	if err := h.adminService.DeleteProxySubscriptionSource(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Subscription deleted successfully"})
}

func (h *ProxyHandler) SyncProxySubscription(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid subscription ID")
		return
	}
	result, err := h.adminService.SyncProxySubscriptionSource(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// GetStats handles getting proxy statistics
// GET /api/v1/admin/proxies/:id/stats
func (h *ProxyHandler) GetStats(c *gin.Context) {
	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}

	// Return mock data for now
	_ = proxyID
	response.Success(c, gin.H{
		"total_accounts":  0,
		"active_accounts": 0,
		"total_requests":  0,
		"success_rate":    100.0,
		"average_latency": 0,
	})
}

// GetProxyAccounts handles getting accounts using a proxy
// GET /api/v1/admin/proxies/:id/accounts
func (h *ProxyHandler) GetProxyAccounts(c *gin.Context) {
	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}

	accounts, err := h.adminService.GetProxyAccounts(c.Request.Context(), proxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.ProxyAccountSummary, 0, len(accounts))
	for i := range accounts {
		out = append(out, *dto.ProxyAccountSummaryFromService(&accounts[i]))
	}
	response.Success(c, out)
}

// BatchCreateProxyItem represents a single proxy in batch create request
type BatchCreateProxyItem struct {
	Protocol string `json:"protocol" binding:"required,oneof=http https socks5 socks5h"`
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port" binding:"required,min=1,max=65535"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// BatchCreateRequest represents batch create proxies request
type BatchCreateRequest struct {
	Proxies []BatchCreateProxyItem `json:"proxies" binding:"required,min=1"`
}

// BatchCreate handles batch creating proxies
// POST /api/v1/admin/proxies/batch
func (h *ProxyHandler) BatchCreate(c *gin.Context) {
	var req BatchCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	created := 0
	skipped := 0

	for _, item := range req.Proxies {
		// Trim all string fields
		host := strings.TrimSpace(item.Host)
		protocol := strings.TrimSpace(item.Protocol)
		username := strings.TrimSpace(item.Username)
		password := strings.TrimSpace(item.Password)

		// Check for duplicates (same host, port, username, password)
		exists, err := h.adminService.CheckProxyExists(c.Request.Context(), host, item.Port, username, password)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}

		if exists {
			skipped++
			continue
		}

		// Create proxy with default name
		_, err = h.adminService.CreateProxy(c.Request.Context(), &service.CreateProxyInput{
			Name:     "default",
			Protocol: protocol,
			Host:     host,
			Port:     item.Port,
			Username: username,
			Password: password,
		})
		if err != nil {
			// If creation fails due to duplicate, count as skipped
			skipped++
			continue
		}

		created++
	}

	response.Success(c, gin.H{
		"created": created,
		"skipped": skipped,
	})
}
