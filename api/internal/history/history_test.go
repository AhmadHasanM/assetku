package history_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/google/uuid"

	"assetku/api/internal/asset"
	"assetku/api/internal/auth"
	"assetku/api/internal/history"
	"assetku/api/internal/httpx"
	"assetku/api/internal/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.Main(m)) }

type page struct {
	Items []struct {
		ID        int64          `json:"id"`
		Event     string         `json:"event"`
		ActorName string         `json:"actor_name"`
		Detail    map[string]any `json:"detail"`
		CreatedAt string         `json:"created_at"`
	} `json:"items"`
	NextBefore *int64 `json:"next_before"`
}

func TestHistoryPaginationAndCrossTenant404(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewDB(t)
	as := auth.NewService(pool, false)
	assets := asset.NewService(pool, as)
	hist := history.NewService(pool, as)
	srv := httptest.NewServer(httpx.NewRouter(httpx.Deps{Pool: pool, Register: func(m *http.ServeMux) {
		as.Register(m)
		assets.Register(m)
		hist.Register(m)
	}}))
	t.Cleanup(srv.Close)
	tenantA := testutil.CreateTenant(t, pool, "A")
	tenantB := testutil.CreateTenant(t, pool, "B")
	userA := testutil.CreateUser(t, pool, tenantA, "a@a.test", "password-1")
	testutil.CreateUser(t, pool, tenantB, "b@b.test", "password-1")

	p := auth.Principal{UserID: userA, TenantID: tenantA, Name: "Ani"}
	a, err := assets.Create(ctx, p, asset.Input{Name: "Laptop", Category: "it_equipment", Status: "available"})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 44 {
		err := history.Record(ctx, pool, history.Entry{TenantID: tenantA, AssetID: a.ID, Event: history.EventLabelPrinted,
			ActorID: &userA, ActorName: "Ani", Detail: map[string]any{"n": i}})
		if err != nil {
			t.Fatal(err)
		}
	}
	// nil Detail is stored as {}.
	if err := history.Record(ctx, pool, history.Entry{TenantID: tenantA, AssetID: a.ID, Event: history.EventBastPrinted, ActorName: "Sistem"}); err != nil {
		t.Fatal(err)
	}

	ca := testutil.Client(t, srv.URL)
	cb := testutil.Client(t, srv.URL)
	for c, em := range map[*testutil.APIClient]string{ca: "a@a.test", cb: "b@b.test"} {
		res := c.Login(em, "password-1")
		res.Body.Close()
	}

	var all []int64
	path := "/api/assets/" + a.ID.String() + "/history"
	sizes := []int{}
	q := ""
	for range 5 {
		res := ca.Do("GET", path+q, nil)
		if res.StatusCode != 200 {
			res.Body.Close()
			t.Fatalf("page %d", res.StatusCode)
		}
		var pg page
		ca.JSON(res, &pg)
		sizes = append(sizes, len(pg.Items))
		for _, it := range pg.Items {
			all = append(all, it.ID)
		}
		if pg.NextBefore == nil {
			break
		}
		if *pg.NextBefore != pg.Items[len(pg.Items)-1].ID {
			t.Fatalf("next_before %d, last item %d", *pg.NextBefore, pg.Items[len(pg.Items)-1].ID)
		}
		q = "?before=" + strconv.FormatInt(*pg.NextBefore, 10)
	}
	if len(sizes) != 3 || sizes[0] != 20 || sizes[1] != 20 || sizes[2] != 6 {
		t.Fatalf("page sizes %v", sizes)
	}
	for i := 1; i < len(all); i++ {
		if all[i] >= all[i-1] {
			t.Fatalf("not id desc at %d: %v", i, all)
		}
	}

	var first page
	res := ca.Do("GET", path, nil)
	ca.JSON(res, &first)
	if it := first.Items[0]; it.Event != "bast_printed" || it.ActorName != "Sistem" || it.Detail == nil || len(it.Detail) != 0 || it.CreatedAt == "" {
		t.Fatalf("newest %+v", it)
	}
	if it := first.Items[1]; it.Event != "label_printed" || it.Detail["n"] != float64(43) {
		t.Fatalf("second %+v", it)
	}

	for _, c := range []struct {
		client *testutil.APIClient
		path   string
		want   int
	}{
		{cb, path, 404},
		{ca, "/api/assets/" + uuid.NewString() + "/history", 404},
		{ca, "/api/assets/nope/history", 404},
		{ca, path + "?before=abc", 422},
	} {
		res := c.client.Do("GET", c.path, nil)
		res.Body.Close()
		if res.StatusCode != c.want {
			t.Fatalf("%s: %d, want %d", c.path, res.StatusCode, c.want)
		}
	}
}
