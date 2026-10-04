# Navigation & Menu Management (CRUDBooster Menu Studio)

TGo Booster features a powerful **Hierarchical Navigation & Menu Management Studio** ala **Laravel CRUDBooster**, allowing administrators to visually organize sidebar navigation with multi-level sub-menus, section groups, and granular role-based access.

---

## 1. Overview & Capabilities

Mounted at `/admin/menus`, the Menu Management Studio delivers:

1. **Hierarchical Tree & Sub-menus**:
   - Supports nesting child items under **Dropdown Folders** (`Type: dropdown`).
   - In the sidebar, dropdowns behave as animated accordions: clicking them expands/collapses the sub-menus with smooth caret rotation and tree guide lines.
   - Automatically detects active routes: if a user visits a nested child route, the parent folder opens and highlights the active sub-menu automatically.
2. **Section Groupings (`Type: header`)**:
   - Organize menus into clear sections (e.g., `Platform`, `Business Modules`, `Finance & Billing`, `Engine Tools`).
   - Renders as sleek uppercase dividers in both the admin sidebar and Command Palette (Ctrl+K).
3. **Four Menu Types**:
   - **📦 Module**: Directly links to any registered CRUD Controller (`products`, `customers`, `orders`, etc.). Selecting a controller automatically auto-suggests the Title, Route Path, and Icon.
   - **📂 Dropdown Folder**: Accordion folder container holding nested sub-menus.
   - **🔗 Custom URL**: Internal routes or external links with configurable `target` (`_self` vs `_blank`).
   - **🏷️ Section Header**: Group divider for visual structure.
4. **Visual SVG Icon Picker**:
   - Curated grid of 20+ modern Lucide-style SVG icons (Dashboard, Package, ShoppingCart, Users, Folder, FileText, Layers, Tag, BarChart, Settings, Shield, Database, Mail, Clock, Key, CreditCard, Sparkles, Truck, Globe, Bell, etc.).
   - Searchable / filterable icon search bar with live preview box.
   - Raw custom `<svg ...></svg>` code toggle for custom enterprise brand icons.
5. **Pill Badges & Color Themes**:
   - Optional badge text on any menu or sub-menu (e.g., `NEW`, `HOT`, `PRO`, or dynamic counters).
   - 5 color themes: **Primary** (Sky Blue), **Success** (Emerald Green), **Warning** (Amber Orange), **Danger** (Rose Red), and **Purple** (Violet/PRO).
6. **Role-Based Privilege Access**:
   - Assign visibility to specific roles (e.g. `Super Administrator`, `Operations Manager`, `Read-Only Auditor`) or toggle "Visible to All Roles".
   - Non-permitted menu items and empty folders are automatically filtered out from non-superadmin sidebars.
7. **Interactive Studio with Live Sidebar Mockup**:
   - Two-column studio layout: interactive hierarchical tree on the left, live sidebar preview on the right.
   - One-click ordering: Move Up (▲) and Move Down (▼) for both top-level and nested sub-menu items.
8. **Persistent JSON Storage**:
   - Persists the entire menu hierarchy to `data/menus.json` (or `starter/data/menus.json`).
   - Auto-initializes on startup with standard recommended groupings if no custom file exists.
   - "Reset Defaults" action lets administrators restore the standard tree at any time.

---

## 2. Menu Item Data Model

Declared in [`pkg/cb/types.go`](file:///d:/Projects/tgo/tgo-booster/pkg/cb/types.go):

```go
// MenuItem represents a dynamic sidebar menu item supporting groups and sub-menus
type MenuItem struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Type        string      `json:"type"` // "module", "url", "dropdown", "header"
	Icon        string      `json:"icon"`
	Path        string      `json:"path"`
	Target      string      `json:"target,omitempty"`       // "_self", "_blank"
	Badge       string      `json:"badge,omitempty"`
	BadgeColor  string      `json:"badge_color,omitempty"` // "primary", "success", "warning", "danger", "purple"
	ParentID    string      `json:"parent_id,omitempty"`
	Order       int         `json:"order"`
	Roles       []string    `json:"roles,omitempty"`        // List of allowed role slugs (empty or ["*"] = all roles)
	PrivilegeID string      `json:"privilege_id,omitempty"` // For backward compatibility
	Children    []*MenuItem `json:"children,omitempty"`
	IsActive    bool        `json:"is_active,omitempty"`
	IsOpen      bool        `json:"is_open,omitempty"`
}
```

### Badge Color Mapping
`BadgeColorClass()` resolves badge color tokens to CSS classes:
- `"primary"` -> `cb-badge-primary`
- `"success"` -> `cb-badge-success`
- `"warning"` -> `cb-badge-warning`
- `"danger"` -> `cb-badge-danger`
- `"purple"` or `"pro"` -> `cb-badge-pro`

---

## 3. Hierarchy & Grouping Example

Here is an example structure saved in `data/menus.json`:

```json
[
  {
    "id": "group_platform",
    "title": "Platform",
    "type": "header",
    "order": 1
  },
  {
    "id": "dashboard",
    "title": "Executive Dashboard",
    "type": "url",
    "path": "/admin",
    "order": 2
  },
  {
    "id": "group_business",
    "title": "Business Modules",
    "type": "header",
    "order": 3
  },
  {
    "id": "catalog_folder",
    "title": "Catalog & Inventory",
    "type": "dropdown",
    "badge": "HOT",
    "badge_color": "warning",
    "order": 4,
    "children": [
      {
        "id": "products",
        "title": "Products",
        "type": "module",
        "path": "/admin/products",
        "parent_id": "catalog_folder",
        "order": 1
      },
      {
        "id": "categories",
        "title": "Categories",
        "type": "module",
        "path": "/admin/categories",
        "parent_id": "catalog_folder",
        "order": 2
      }
    ]
  },
  {
    "id": "customers",
    "title": "Customers",
    "type": "module",
    "path": "/admin/customers",
    "order": 5
  },
  {
    "id": "orders",
    "title": "Orders",
    "type": "module",
    "path": "/admin/orders",
    "order": 6
  }
]
```

---

## 4. API & Studio Endpoints

All endpoints are registered under `{{.AdminPath}}/menus` (e.g. `/admin/menus`) and require Superadmin privileges:

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/admin/menus` | Renders the Menu Management Studio interface |
| `GET` | `/admin/menus/api/menu?id=<id>` | Fetches single menu item JSON for the edit modal |
| `POST` | `/admin/menus/save` | Creates or updates a menu item (JSON or Form payload) |
| `POST` | `/admin/menus/delete?id=<id>` | Deletes a menu item and any nested sub-menus |
| `POST` | `/admin/menus/reorder?id=<id>&direction=up\|down` | Swaps position of a menu item with its sibling |
| `POST` | `/admin/menus/reset` | Resets navigation structure to default system configuration |

---

## 5. Hot-Reload & Module Generator Integration

When a new CRUD module is generated using the **Module Generator** (`/admin/module_generator`):
1. `e.RegisterDynamic(ctrl)` automatically calls `e.syncControllerMenu(ctrl)`.
2. The engine detects if the module is already in `e.menus`. If not, it automatically inserts it under the `"group_business"` section header or appends it to the root navigation.
3. `e.SaveMenus()` writes the update to disk immediately.
4. The new module appears live in the sidebar without requiring an application restart.
