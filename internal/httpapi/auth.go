package httpapi

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/jfebiyanto/easyPOS-Go/internal/identity"
	"github.com/jfebiyanto/easyPOS-Go/internal/platform/httpx"
)

type meResponse struct {
	ID          uuid.UUID                      `json:"id"`
	Name        string                         `json:"name"`
	Username    string                         `json:"username"`
	Email       *string                        `json:"email"`
	Role        string                         `json:"role"`
	StoreID     *uuid.UUID                     `json:"store_id"`
	ActiveStore *identity.StoreRef             `json:"active_store"`
	Company     identity.Company               `json:"company"`
	Permissions map[string]identity.Permission `json:"permissions"`
}

func (s *server) buildMe(r *http.Request, sess *identity.Session) (meResponse, error) {
	u := sess.User
	co, err := s.Identity.Company(r.Context(), u.CompanyID)
	if err != nil {
		return meResponse{}, err
	}
	m := meResponse{ID: u.ID, Name: u.Name, Username: u.Username, Role: u.Role, ActiveStore: sess.ActiveStore,
		Company: co, Permissions: identity.Permissions(u.Role)}
	if u.Email.Valid {
		m.Email = &u.Email.String
	}
	if u.StoreID.Valid {
		m.StoreID = &u.StoreID.UUID
	}
	return m, nil
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		DeviceName string `json:"device_name"`
		Client     string `json:"client"`
	}
	if err := httpx.Decode(r, &in); err != nil || in.Username == "" || in.Password == "" || !identity.ClientKinds[in.Client] {
		httpx.Error(w, http.StatusUnprocessableEntity, "username, password, dan client (backoffice|pos|api) wajib diisi.")
		return
	}
	res, err := s.Identity.Login(r.Context(), identity.LoginInput{Username: in.Username, Password: in.Password, DeviceName: in.DeviceName, Client: in.Client})
	switch {
	case errors.Is(err, identity.ErrInvalidCredentials):
		httpx.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	case errors.Is(err, identity.ErrForbidden):
		httpx.Error(w, http.StatusForbidden, err.Error())
		return
	case err != nil:
		s.Log.Error("login", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "Terjadi kesalahan server.")
		return
	}
	me, err := s.buildMe(r, &res.Session)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Terjadi kesalahan server.")
		return
	}
	out := map[string]any{"user": me}
	if in.Client == "api" {
		out["token"] = res.Token
	} else {
		http.SetCookie(w, &http.Cookie{Name: CookieName, Value: res.Token, Path: "/", HttpOnly: true,
			SameSite: http.SameSiteLaxMode, Secure: s.CookieSecure})
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	me, err := s.buildMe(r, sessionFrom(r.Context()))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Terjadi kesalahan server.")
		return
	}
	httpx.JSON(w, http.StatusOK, me)
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.Identity.Logout(r.Context(), sessionFrom(r.Context())); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Terjadi kesalahan server.")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: s.CookieSecure})
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) accessibleStores(w http.ResponseWriter, r *http.Request) {
	list, err := s.Identity.Accessible(r.Context(), sessionFrom(r.Context()).User)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Terjadi kesalahan server.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"stores": list})
}

func (s *server) switchStore(w http.ResponseWriter, r *http.Request) {
	var in struct {
		StoreID string `json:"store_id"`
	}
	id, perr := uuid.Parse("")
	if err := httpx.Decode(r, &in); err == nil {
		id, perr = uuid.Parse(in.StoreID)
	}
	if perr != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "store_id tidak valid.")
		return
	}
	st, err := s.Identity.SwitchStore(r.Context(), sessionFrom(r.Context()), id)
	if errors.Is(err, identity.ErrForbidden) {
		httpx.Error(w, http.StatusForbidden, "Anda tidak punya akses ke gerai ini.")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "Terjadi kesalahan server.")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"active_store": st})
}
