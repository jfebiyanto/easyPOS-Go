package identity

import "strings"

// Peran pengguna (kolom users.role).
const (
	RoleOwner        = "owner"
	RoleStoreManager = "store_manager"
	RoleCashier      = "cashier"
)

// Port dari app/Support/BackofficeAccess.php (Laravel).
var (
	// Tertutup total untuk store_manager (baca dan tulis).
	closedPrefixes = []string{"admin.company.", "admin.numberings.", "admin.sap-import.", "admin.sap-postings.", "admin.settings."}
	// Boleh ditulis store_manager (data tetap dibatasi ke gerainya).
	writablePrefixes = []string{"admin.eod.", "admin.stock-review.", "admin.stock-adjustments.", "admin.stock-transfer-shipments.", "admin.stock-transfer-receipts.", "admin.stock-transfer-returns.", "admin.deliveries."}
	// Rute .edit yang sebenarnya halaman utama, bukan formulir ubah.
	readableForms = []string{"admin.promotion-hierarchy.edit"}
)

func hasPrefix(key string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

func isForm(key string) bool {
	for _, k := range readableForms {
		if k == key {
			return false
		}
	}
	return strings.HasSuffix(key, ".create") || strings.HasSuffix(key, ".edit")
}

// Allows: bolehkah peran mengakses route key dengan metode HTTP tertentu.
func Allows(role, routeKey, method string) bool {
	switch role {
	case RoleOwner:
		return true
	case RoleStoreManager:
	default:
		return false
	}
	isRead := method == "GET" || method == "HEAD"
	if routeKey == "" {
		return isRead
	}
	if hasPrefix(routeKey, closedPrefixes) {
		return false
	}
	if hasPrefix(routeKey, writablePrefixes) {
		return true
	}
	return isRead && !isForm(routeKey)
}

// ReadOnly: halaman hanya baca bagi peran ini (tombol tulis disembunyikan).
func ReadOnly(role, routeKey string) bool {
	if role == RoleOwner {
		return false
	}
	return routeKey == "" || !hasPrefix(routeKey, writablePrefixes)
}

// Permission adalah entri me.permissions per route key.
type Permission struct {
	Visible  bool `json:"visible"`
	ReadOnly bool `json:"readOnly"`
}

// MenuRoutes: route key halaman (GET) yang menjadi item menu backoffice.
var MenuRoutes = []string{
	"admin.dashboard.index",
	"admin.sales.index", "admin.sales-returns.index",
	"admin.products.index", "admin.price-lists.index", "admin.customers.index", "admin.promotions.index",
	"admin.stock-review.index", "admin.stock-adjustments.index", "admin.stock-transfer-shipments.index",
	"admin.eod.index", "admin.numberings.index", "admin.settings-view.index", "admin.company.edit",
}

func Permissions(role string) map[string]Permission {
	m := make(map[string]Permission, len(MenuRoutes))
	for _, k := range MenuRoutes {
		m[k] = Permission{Visible: Allows(role, k, "GET"), ReadOnly: ReadOnly(role, k)}
	}
	return m
}
