package cb

import (
	"net/http/httptest"
	"sync"
	"testing"
)

// Pembaca peran/menu (tiap request) berjalan bersamaan dengan muat-ulang dokumen yang mengganti slice; harus bersih
// di bawah -race.
func TestRolesMenusConcurrentReloadIsRaceFree(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Uji")
	fs := &failDocStore{}
	if err := e.SetDocStore(fs); err != nil {
		t.Fatal(err)
	}
	u := &User{ID: "u", Name: "U", Email: "u@uji.id", PrivilegeID: "2", RoleName: "Operations Manager"}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				uu := *u // salinan: SetSessionUser mengisi CreatedAt
				_ = e.FindRole("2")
				_ = e.CheckPermission(sessionReq(e, &uu), "users", "read")
				_ = e.buildActiveMenus(httptest.NewRequest("GET", "/admin", nil), "/admin", u)
			}
		}()
	}
	for i := 0; i < 200; i++ {
		e.reloadDoc(DocPrivileges)
		e.reloadDoc(DocMenus)
		_ = e.SaveRoles()
		_ = e.SaveMenus()
	}
	close(stop)
	wg.Wait()
}
