package identity

import "testing"

func TestAllows(t *testing.T) {
	cases := []struct {
		role, key, method string
		want              bool
	}{
		// owner selalu boleh
		{RoleOwner, "admin.company.edit", "GET", true},
		{RoleOwner, "admin.settings.update", "PUT", true},
		{RoleOwner, "", "DELETE", true},
		// selain owner/store_manager ditolak
		{RoleCashier, "admin.products.index", "GET", false},
		{"lainnya", "admin.eod.index", "GET", false},
		// store_manager: prefix CLOSED tertutup total
		{RoleStoreManager, "admin.company.edit", "GET", false},
		{RoleStoreManager, "admin.numberings.index", "GET", false},
		{RoleStoreManager, "admin.numberings.update", "PUT", false},
		{RoleStoreManager, "admin.sap-import.index", "GET", false},
		{RoleStoreManager, "admin.sap-postings.index", "GET", false},
		{RoleStoreManager, "admin.settings.update", "PUT", false},
		// route key baca khusus Konfigurasi Sistem di luar prefix CLOSED
		{RoleStoreManager, "admin.settings-view.index", "GET", true},
		{RoleStoreManager, "admin.settings-view.index", "PUT", false},
		// prefix WRITABLE boleh tulis
		{RoleStoreManager, "admin.eod.store", "POST", true},
		{RoleStoreManager, "admin.stock-review.index", "GET", true},
		{RoleStoreManager, "admin.stock-adjustments.store", "POST", true},
		{RoleStoreManager, "admin.stock-adjustments.create", "GET", true}, // WRITABLE menang atas aturan form
		{RoleStoreManager, "admin.stock-transfer-shipments.update", "PUT", true},
		{RoleStoreManager, "admin.stock-transfer-receipts.store", "POST", true},
		{RoleStoreManager, "admin.stock-transfer-returns.store", "POST", true},
		{RoleStoreManager, "admin.deliveries.update", "PATCH", true},
		// selain itu hanya baca, bukan form
		{RoleStoreManager, "admin.products.index", "GET", true},
		{RoleStoreManager, "admin.products.index", "HEAD", true},
		{RoleStoreManager, "admin.products.show", "GET", true},
		{RoleStoreManager, "admin.products.store", "POST", false},
		{RoleStoreManager, "admin.products.update", "PUT", false},
		{RoleStoreManager, "admin.products.destroy", "DELETE", false},
		{RoleStoreManager, "admin.products.create", "GET", false},
		{RoleStoreManager, "admin.products.edit", "GET", false},
		{RoleStoreManager, "admin.promotion-hierarchy.edit", "GET", true},
		{RoleStoreManager, "admin.promotion-hierarchy.update", "PUT", false},
		// tanpa route key: hanya baca
		{RoleStoreManager, "", "GET", true},
		{RoleStoreManager, "", "POST", false},
	}
	for _, c := range cases {
		if got := Allows(c.role, c.key, c.method); got != c.want {
			t.Errorf("Allows(%q, %q, %s) = %v, want %v", c.role, c.key, c.method, got, c.want)
		}
	}
}

func TestReadOnlyDanPermissions(t *testing.T) {
	if ReadOnly(RoleOwner, "admin.products.index") {
		t.Error("owner tidak pernah read-only")
	}
	if ReadOnly(RoleStoreManager, "admin.eod.index") {
		t.Error("eod writable bagi store_manager")
	}
	if !ReadOnly(RoleStoreManager, "admin.products.index") || !ReadOnly(RoleStoreManager, "") {
		t.Error("produk dan rute tanpa key read-only bagi store_manager")
	}
	p := Permissions(RoleStoreManager)
	if p["admin.numberings.index"].Visible || p["admin.company.edit"].Visible {
		t.Error("menu CLOSED tidak boleh terlihat")
	}
	if !p["admin.settings-view.index"].Visible || !p["admin.settings-view.index"].ReadOnly {
		t.Error("settings-view terlihat dan read-only")
	}
	if !Permissions(RoleOwner)["admin.numberings.index"].Visible {
		t.Error("owner melihat semua")
	}
	for k, v := range Permissions(RoleCashier) {
		if v.Visible {
			t.Errorf("cashier tidak boleh melihat %s", k)
		}
	}
}
