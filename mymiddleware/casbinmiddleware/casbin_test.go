package casbinmiddleware

import (
	"context"
	"errors"
	"google.golang.org/grpc"
	"learn_together/api/authcontrol"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func getEnforcer() (*authcontrol.Enforcer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cc, err := authcontrol.NewClient(ctx, "127.0.0.1:2379", "auth_service", grpc.WithInsecure())
	if err != nil {
		panic(err)
	}

	enforcer, err := cc.NewEnforcer(ctx, authcontrol.Config{ModelText: ""})
	return enforcer,nil
}
func testRequest(t *testing.T, h echo.HandlerFunc, user string, path string, method string, code int) {
	//新建一个echo
	e := echo.New()
	//新建一个请求
	req := httptest.NewRequest(method, path, nil)
	req.SetBasicAuth(user, "secret")
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	//模拟一个请求 进入echo.HandlerFunc
	err := h(c)

	if err != nil {
		if errObj, ok := err.(*echo.HTTPError); ok {
			if errObj.Code != code {
				t.Errorf("%s, %s, %s: %d, supposed to be %d", user, path, method, errObj.Code, code)
			}
		} else {
			t.Error(err)
		}
	} else {
		if c.Response().Status != code {
			t.Errorf("%s, %s, %s: %d, supposed to be %d", user, path, method, c.Response().Status, code)
		}
	}
}

func TestAuth(t *testing.T) {
	ce, _ := getEnforcer()
	h := Middleware(ce)(func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	testRequest(t, h, "alice", "/dataset1/resource1", echo.GET, 200)
	//testRequest(t, h, "alice", "/dataset1/resource1", echo.POST, 200)
	//testRequest(t, h, "alice", "/dataset1/resource2", echo.GET, 200)
	//testRequest(t, h, "alice", "/dataset1/resource2", echo.POST, 403)
}

func TestPathWildcard(t *testing.T) {
	ce, _ := getEnforcer()
	h := Middleware(ce)(func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	testRequest(t, h, "bob", "/dataset2/resource1", "GET", 200)
	testRequest(t, h, "bob", "/dataset2/resource1", "POST", 200)
	testRequest(t, h, "bob", "/dataset2/resource1", "DELETE", 200)
	testRequest(t, h, "bob", "/dataset2/resource2", "GET", 200)
	testRequest(t, h, "bob", "/dataset2/resource2", "POST", 403)
	testRequest(t, h, "bob", "/dataset2/resource2", "DELETE", 403)

	testRequest(t, h, "bob", "/dataset2/folder1/item1", "GET", 403)
	testRequest(t, h, "bob", "/dataset2/folder1/item1", "POST", 200)
	testRequest(t, h, "bob", "/dataset2/folder1/item1", "DELETE", 403)
	testRequest(t, h, "bob", "/dataset2/folder1/item2", "GET", 403)
	testRequest(t, h, "bob", "/dataset2/folder1/item2", "POST", 200)
	testRequest(t, h, "bob", "/dataset2/folder1/item2", "DELETE", 403)
}

func TestRBAC(t *testing.T) {
	ce, _ := getEnforcer()
	h := Middleware(ce)(func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	// cathy can access all /dataset1/* resources via all methods because it has the dataset1_admin role.
	testRequest(t, h, "cathy", "/dataset1/item", "GET", 200)
	testRequest(t, h, "cathy", "/dataset1/item", "POST", 200)
	testRequest(t, h, "cathy", "/dataset1/item", "DELETE", 200)
	testRequest(t, h, "cathy", "/dataset2/item", "GET", 403)
	testRequest(t, h, "cathy", "/dataset2/item", "POST", 403)
	testRequest(t, h, "cathy", "/dataset2/item", "DELETE", 403)

	// delete all roles on user cathy, so cathy cannot access any resources now.
	ce.DeleteRolesForUser(context.Background(), "cathy")

	testRequest(t, h, "cathy", "/dataset1/item", "GET", 403)
	testRequest(t, h, "cathy", "/dataset1/item", "POST", 403)
	testRequest(t, h, "cathy", "/dataset1/item", "DELETE", 403)
	testRequest(t, h, "cathy", "/dataset2/item", "GET", 403)
	testRequest(t, h, "cathy", "/dataset2/item", "POST", 403)
	testRequest(t, h, "cathy", "/dataset2/item", "DELETE", 403)
}

func TestEnforceError(t *testing.T) {
	ce, _ := getEnforcer()
	h := Middleware(ce)(func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	testRequest(t, h, "cathy", "/dataset1/item", "GET", 500)
}

func TestCustomUserGetter(t *testing.T) {
	ce, _ := getEnforcer()
	cnf := Config{
		Skipper:  middleware.DefaultSkipper,
		Enforcer: ce,
		UserGetter: func(c echo.Context) (string, error) {
			return "not_cathy_at_all", nil
		},
	}
	h := MiddlewareWithConfig(cnf)(func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})
	testRequest(t, h, "cathy", "/dataset1/item", "GET", 403)
}

func TestUserGetterError(t *testing.T) {
	ce, _ := getEnforcer()
	cnf := Config{
		Skipper:  middleware.DefaultSkipper,
		Enforcer: ce,
		UserGetter: func(c echo.Context) (string, error) {
			return "", errors.New("no idea who you are")
		},
	}
	h := MiddlewareWithConfig(cnf)(func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})
	testRequest(t, h, "cathy", "/dataset1/item", "GET", 403)
}
