# Privileges & Roles Management (CRUDBooster RBAC Matrix)

TGo Booster features a Role-Based Access Control (RBAC) permissions matrix ala **Laravel CRUDBooster**, providing granular authorization controls per module without messy config code.

---

## 1. Overview & Capabilities

The Privileges Studio is mounted at `/admin/privileges` and provides:
1. **Granular Module Matrix**: Configure permissions per module:
   - **Is Visible**: Controls menu visibility in the sidebar.
   - **Can Create**: Controls record creation (`/add` or `/create`).
   - **Can Read**: Controls grid viewing (`/`) and JSON retrieval (`/data`, `/detail/:id`).
   - **Can Update**: Controls edit form access and updates (`/edit/:id`).
   - **Can Delete**: Controls record deletion (`/delete/:id`) and bulk actions (`/bulk-action`).
2. **Superadmin Bypass**: A toggle that grants unrestricted administrative rights across all modules, matrix configurations, and engine settings.
3. **Live Session Role Switcher**: Instant session switcher in the UI and via API (`/admin/privileges/switch?role=<slug>`) allowing developers to test any permission rule in real time without having to re-login.
4. **403 Forbidden Screen**: Styled executive access denied card informing users of the exact permission missing (`can_create`, `can_update`, etc.) and directing them back safely.
5. **Persistent Storage**: Role definitions and granular matrix rules persist to `data/privileges.json` (or `starter/data/privileges.json`).

---

## 2. Default System Roles

Booster provides 3 preconfigured roles out of the box:

| Role Name | Slug | Superadmin | Scope | Default Permissions |
|-----------|------|------------|-------|---------------------|
| **Super Administrator** | `superadmin` | ★ Yes | Global | Full Unrestricted Access (C, R, U, D, Visible) across all modules & tools |
| **Operations Manager** | `ops_manager` | No | Operational | Read & Write (`can_create`, `can_read`, `can_update`, `is_visible`), deletion blocked |
| **Read-Only Auditor** | `auditor` | No | Compliance | Inspection only (`can_read`, `is_visible`), no creation, modification, or deletion |

---

## 3. Data Structures & Go Types

Declared in [pkg/cb/types.go](file:///d:/Projects/tgo/tgo-booster/pkg/cb/types.go):

```go
// PermissionMatrix represents granular CRUD permissions for a module
type PermissionMatrix struct {
    IsVisible bool `json:"is_visible"`
    CanCreate bool `json:"can_create"`
    CanRead   bool `json:"can_read"`
    CanUpdate bool `json:"can_update"`
    CanDelete bool `json:"can_delete"`
}

// Role represents a Privilege Role with its module permissions matrix
type Role struct {
    ID           string                      `json:"id"`
    Name         string                      `json:"name"`
    Slug         string                      `json:"slug"`
    IsSuperadmin bool                        `json:"is_superadmin"`
    Description  string                      `json:"description"`
    UsersCount   int                         `json:"users_count"`
    Permissions  map[string]PermissionMatrix `json:"permissions"` // map[moduleTable]PermissionMatrix
    CreatedAt    time.Time                   `json:"created_at"`
}

// CanAccess evaluates whether a role is permitted to perform an action on a module
func (r *Role) CanAccess(moduleTable, action string) bool {
    if r == nil || r.IsSuperadmin || strings.EqualFold(r.Name, "superadmin") || strings.EqualFold(r.Slug, "superadmin") {
        return true
    }
    ...
}
```

---

## 4. Permission Enforcement in HTTP Requests

In [pkg/cb/controller.go](file:///d:/Projects/tgo/tgo-booster/pkg/cb/controller.go), each request in `ServeHTTP` maps to an action (`read`, `create`, `update`, `delete`):

```go
action := "read"
switch {
case path == "add" || path == "create":
    action = "create"
case strings.HasPrefix(path, "edit/"):
    action = "update"
case strings.HasPrefix(path, "delete/") || path == "bulk-action":
    action = "delete"
}

if c.Engine != nil && !c.Engine.CheckPermission(r, c.Table, action) {
    if r.Header.Get("Accept") == "application/json" || strings.HasSuffix(r.URL.Path, "/data") || r.Method == http.MethodPost {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusForbidden)
        _ = json.NewEncoder(w).Encode(map[string]interface{}{
            "success": false,
            "error":   fmt.Sprintf("Access Denied (403): Role does not have '%s' permission on %s", action, c.Title),
        })
        return
    }
    c.Engine.RenderForbidden(w, r, c.Title, action)
    return
}
```

---

## 5. HTTP Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/admin/privileges` | Renders the CRUDBooster Privileges Studio & matrix manager |
| `GET` | `/admin/privileges/api/role?id=<id>` | Fetches JSON definition and matrix of a specific role |
| `POST` | `/admin/privileges/save` | Creates or updates a role and its module permissions matrix |
| `POST` | `/admin/privileges/delete?id=<id>` | Deletes a custom role (Superadmin cannot be deleted) |
| `POST` | `/admin/privileges/switch?role=<slug>` | Live role switcher: switches active session user's role |
