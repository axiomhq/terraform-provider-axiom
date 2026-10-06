package axiom

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/axiomhq/axiom-go/axiom"
)

// fakeRBACAPI is an in-memory stand-in for the Axiom roles, groups and users
// APIs. It reproduces the server behavior that affects Terraform state:
//   - role members are derived from users' base roles and ignored on write
//   - dataset and view entries without capabilities are dropped
//   - deleting a role resets its users to "none" and removes it from groups
type fakeRBACAPI struct {
	mu     sync.Mutex
	nextID int
	roles  map[string]*axiom.Role
	groups map[string]*axiom.Group
	users  map[string]*axiom.User
}

var builtinRoles = []string{"owner", "admin", "user", "read-only", "none"}

func newFakeRBACAPI(t *testing.T, users ...*axiom.User) *httptest.Server {
	t.Helper()

	api := &fakeRBACAPI{
		roles:  map[string]*axiom.Role{},
		groups: map[string]*axiom.Group{},
		users:  map[string]*axiom.User{},
	}
	for _, u := range users {
		api.users[u.ID] = u
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v2/rbac/roles", api.listRoles)
	mux.HandleFunc("POST /v2/rbac/roles", api.createRole)
	mux.HandleFunc("GET /v2/rbac/roles/{id}", api.getRole)
	mux.HandleFunc("PUT /v2/rbac/roles/{id}", api.updateRole)
	mux.HandleFunc("DELETE /v2/rbac/roles/{id}", api.deleteRole)
	mux.HandleFunc("GET /v2/rbac/groups/{id}", api.getGroup)
	mux.HandleFunc("POST /v2/rbac/groups", api.createGroup)
	mux.HandleFunc("PUT /v2/rbac/groups/{id}", api.updateGroup)
	mux.HandleFunc("DELETE /v2/rbac/groups/{id}", api.deleteGroup)
	mux.HandleFunc("GET /v2/users", api.listUsers)
	mux.HandleFunc("POST /v2/users", api.createUser)
	mux.HandleFunc("GET /v2/users/{id}", api.getUser)
	mux.HandleFunc("DELETE /v2/users/{id}", api.deleteUser)
	mux.HandleFunc("PUT /v2/users/{id}/role", api.updateUserRole)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv
}

func (api *fakeRBACAPI) id(prefix string) string {
	api.nextID++
	return fmt.Sprintf("%s-%d", prefix, api.nextID)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"message": msg})
}

// hasCapabilities reports whether a capabilities struct grants anything. All
// capability fields are omitempty, so an empty struct encodes as "{}".
func hasCapabilities(v any) bool {
	b, _ := json.Marshal(v)
	return string(b) != "{}"
}

func dropEmpty[T any](m map[string]T) map[string]T {
	result := map[string]T{}
	for k, v := range m {
		if hasCapabilities(v) {
			result[k] = v
		}
	}
	return result
}

func (api *fakeRBACAPI) roleWithMembers(role *axiom.Role) *axiom.Role {
	r := *role
	r.Members = []string{}
	for _, u := range api.users {
		if u.Role.ID == role.ID {
			r.Members = append(r.Members, u.ID)
		}
	}
	slices.Sort(r.Members)
	return &r
}

func (api *fakeRBACAPI) applyRoleRequest(role *axiom.Role, req axiom.RoleRequest) {
	role.Name = req.Name
	role.Description = req.Description
	role.DatasetCapabilities = dropEmpty(req.DatasetCapabilities)
	role.ViewCapabilities = dropEmpty(req.ViewCapabilities)
	role.OrgCapabilities = req.OrgCapabilities
}

func (api *fakeRBACAPI) listRoles(w http.ResponseWriter, _ *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	roles := []*axiom.Role{}
	for _, role := range api.roles {
		roles = append(roles, api.roleWithMembers(role))
	}
	writeJSON(w, http.StatusOK, roles)
}

func (api *fakeRBACAPI) createRole(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	var req axiom.RoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	for _, role := range api.roles {
		if role.Name == req.Name {
			writeError(w, http.StatusBadRequest, "role with that name already exists")
			return
		}
	}

	role := &axiom.Role{ID: api.id("role")}
	api.applyRoleRequest(role, req)
	api.roles[role.ID] = role

	writeJSON(w, http.StatusOK, api.roleWithMembers(role))
}

func (api *fakeRBACAPI) getRole(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	role, ok := api.roles[r.PathValue("id")]
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, api.roleWithMembers(role))
}

func (api *fakeRBACAPI) updateRole(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	role, ok := api.roles[r.PathValue("id")]
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	var req axiom.RoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	api.applyRoleRequest(role, req)

	writeJSON(w, http.StatusOK, api.roleWithMembers(role))
}

func (api *fakeRBACAPI) deleteRole(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	id := r.PathValue("id")
	if _, ok := api.roles[id]; !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	delete(api.roles, id)

	for _, u := range api.users {
		if u.Role.ID == id {
			u.Role.ID, u.Role.Name = "none", "none"
		}
	}
	for _, g := range api.groups {
		g.Roles = slices.DeleteFunc(g.Roles, func(roleID string) bool { return roleID == id })
	}

	w.WriteHeader(http.StatusNoContent)
}

func (api *fakeRBACAPI) applyGroupRequest(group *axiom.Group, req axiom.GroupRequest) {
	group.Name = req.Name
	group.Description = req.Description
	group.Roles = append([]string{}, req.Roles...)
	if !group.IsManaged {
		group.Members = append([]string{}, req.Members...)
	}
}

func (api *fakeRBACAPI) createGroup(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	var req axiom.GroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	group := &axiom.Group{ID: api.id("group")}
	api.applyGroupRequest(group, req)
	api.groups[group.ID] = group

	writeJSON(w, http.StatusOK, group)
}

func (api *fakeRBACAPI) getGroup(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	group, ok := api.groups[r.PathValue("id")]
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, group)
}

func (api *fakeRBACAPI) updateGroup(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	group, ok := api.groups[r.PathValue("id")]
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	var req axiom.GroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	api.applyGroupRequest(group, req)

	writeJSON(w, http.StatusOK, group)
}

func (api *fakeRBACAPI) deleteGroup(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	id := r.PathValue("id")
	if _, ok := api.groups[id]; !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	delete(api.groups, id)

	w.WriteHeader(http.StatusNoContent)
}

func (api *fakeRBACAPI) listUsers(w http.ResponseWriter, _ *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	users := []*axiom.User{}
	for _, u := range api.users {
		users = append(users, u)
	}
	slices.SortFunc(users, func(a, b *axiom.User) int {
		if a.ID < b.ID {
			return -1
		}
		return 1
	})
	writeJSON(w, http.StatusOK, users)
}

func (api *fakeRBACAPI) createUser(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	var req axiom.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	u := &axiom.User{ID: api.id("user"), Name: req.Name, Email: req.Email}
	u.Role.ID, u.Role.Name = req.Role, req.Role
	api.users[u.ID] = u

	writeJSON(w, http.StatusOK, u)
}

func (api *fakeRBACAPI) getUser(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	u, ok := api.users[r.PathValue("id")]
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (api *fakeRBACAPI) deleteUser(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	delete(api.users, r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}

func (api *fakeRBACAPI) updateUserRole(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	u, ok := api.users[r.PathValue("id")]
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	var req axiom.UpdateUserRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	switch role, isCustom := api.roles[req.Role]; {
	case slices.Contains(builtinRoles, req.Role):
		u.Role.ID, u.Role.Name = req.Role, req.Role
	case isCustom:
		u.Role.ID, u.Role.Name = role.ID, role.Name
	default:
		writeError(w, http.StatusBadRequest, "unknown role")
		return
	}

	writeJSON(w, http.StatusOK, u)
}
